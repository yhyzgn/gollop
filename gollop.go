// Copyright 2020 yhyzgn gollop
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// author : 颜洪毅
// e-mail : yhyzgn@gmail.com
// time   : 2020-04-01 10:28 下午
// version: 1.0.0
// desc   : 连接池

package gollop

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// 连接池
type Pool struct {
	opt               *options                   // 配置项
	numClosed         int64                      // 已关闭的连接数量
	mu                sync.Mutex                 // 锁，操作连接池各成员时用到
	freeConn          []Connector                // 空闲连接池
	connRequests      map[int64]chan connRequest // 被阻塞的请求队列
	nextRequestKey    int64                      // 下一个被阻塞的请求key
	numOpened         int                        // 已打开的连接数量
	openerCh          chan struct{}              // 创建新连接信号量
	waitCount         int64                      // 当前等待连接的请求数量
	waitDuration      int64                      // 已出队请求的等待时间总和
	maxIdleClosed     int64                      // 归还连接时，空闲连接数超过最大空闲连接数量时，关闭超出的连接
	maxLifetimeClosed int64                      // 空闲超时需要关闭的连接数量
	closed            bool                       // 连接池是否已被关闭
	stop              func()                     // 停止连接池 context 操作
}

// 创建连接池
func New(ops ...Options) *Pool {
	ctx, cancel := context.WithCancel(context.Background())

	opts := defOptions()
	for _, o := range ops {
		o(opts)
	}

	pl := &Pool{
		opt:          opts,
		connRequests: make(map[int64]chan connRequest),
		openerCh:     make(chan struct{}, connRequestSize),
		closed:       false,
		stop:         cancel,
	}

	// 创建初始化连接
	go pl.openInitialConnections()
	// 协程监视创建连接
	go pl.connectionOpener(ctx)
	// 协程定时清理过时连接
	go pl.startCleanerLocked()

	return pl
}

// 获取一个连接
//
// 优先分配空闲连接
func (p *Pool) Get() (Connector, error) {
	ctx, _ := context.WithTimeout(context.Background(), p.opt.waitTimeout)
	return p.GetWithContext(ctx)
}

// 通过自定义 context 获取一个连接
//
// 优先分配空闲连接
func (p *Pool) GetWithContext(ctx context.Context) (Connector, error) {
	return p.get(ctx, strategyIdleOrNew)
}

// 创建一个连接
//
// 直接创建新连接（如果当前连接数未达到上限）
func (p *Pool) New() (Connector, error) {
	ctx, _ := context.WithTimeout(context.Background(), p.opt.waitTimeout)
	return p.NewWithContext(ctx)
}

// 通过自定义 context 创建一个连接
//
// 直接创建新连接（如果当前连接数未达到上限）
func (p *Pool) NewWithContext(ctx context.Context) (Connector, error) {
	return p.get(ctx, strategyAlwaysNew)
}

// 归还一个连接
func (p *Pool) Put(cn Connector) {
	p.PutWithError(cn, nil)
}

// 归还一个连接，可能是无效连接
func (p *Pool) PutWithError(cn Connector, err error) {
	p.putConn(cn, err)
}

// 当前连接数量
func (p *Pool) OpenedSize() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.numOpened
}

// 当前空闲连接数量
func (p *Pool) IdleSize() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.freeConn)
}

// 获取连接池状态
func (p *Pool) Status() Status {
	wait := atomic.LoadInt64(&p.waitDuration)

	p.mu.Lock()
	defer p.mu.Unlock()

	return Status{
		PoolSize:          p.numOpened,
		MaxPoolSize:       p.opt.maxOpen,
		IdleSize:          len(p.freeConn),
		MaxIdleSize:       p.opt.maxIdle,
		InUseSize:         p.numOpened - len(p.freeConn),
		WaitCount:         p.waitCount,
		WaitDuration:      time.Duration(wait),
		MaxIdleClosed:     p.maxIdleClosed,
		MaxLifetimeClosed: p.maxLifetimeClosed,
		ClosedCount:       p.numClosed,
	}
}

// 关闭连接池
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.closed {
		if p.openerCh != nil {
			close(p.openerCh)
		}

		for _, req := range p.connRequests {
			close(req)
		}
		p.connRequests = nil

		p.freeConn = nil
		p.closed = true

		// 停止连接池
		p.stop()
	}
	return nil
}

// 获取一个连接
//
// 如果当前连接数量已达到上限，则将请求入队等待，并阻塞
func (p *Pool) get(ctx context.Context, strategy connRequestStrategy) (Connector, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, ErrClosedConn
	}

	// 检查 context 过时
	select {
	default:
	case <-ctx.Done():
		p.mu.Unlock()
		return nil, ctx.Err()
	}

	numFree := len(p.freeConn)
	if strategy == strategyIdleOrNew && numFree > 0 {
		// 从空闲连接池中取
		cn := p.freeConn[0]
		p.freeConn = p.freeConn[1:]
		cn.SetInUse(true)
		p.mu.Unlock()

		// 检查取出的连接是否超时
		if p.expired(cn) {
			_ = p.closeConn(cn)
			return nil, ErrBadConn
		}

		// 检查当前连接的锁，确保连接已使用完成，并且无错误发生
		cn.GetLocker().Lock()
		err := cn.GetLastErr()
		cn.GetLocker().Unlock()

		if err != nil {
			_ = p.closeConn(cn)
			return nil, ErrBadConn
		}
		return cn, nil
	}

	// 如果没有空闲连接，并且当前连接数已达到最大连接数，就将当前请求添加到请求队列中等待
	// 并阻塞在这里，直到其它协程将占用的连接释放或connectionOpener创建
	if p.opt.maxOpen > 0 && p.numOpened >= p.opt.maxOpen {
		req := make(chan connRequest, 1)
		reqKey := p.nextRequestKeyLocked()

		p.connRequests[reqKey] = req
		p.waitCount++
		p.mu.Unlock()

		waitStart := now()

		select {
		case <-ctx.Done():
			// 等待超时，从等待队列中移除请求
			p.mu.Lock()
			delete(p.connRequests, reqKey)
			p.mu.Unlock()

			// 总等待时长统计
			atomic.AddInt64(&p.waitDuration, int64(time.Since(waitStart)))

			// 当该请求被分配到连接后，将连接归还给连接池（因为当前请求已经等待超时）
			select {
			default:
			case ret, ok := <-req:
				if ok && ret.conn != nil {
					p.putConn(ret.conn, nil)
				}
			}
			return nil, ctx.Err()
		case ret, ok := <-req:
			// 总等待时长统计
			atomic.AddInt64(&p.waitDuration, int64(time.Since(waitStart)))

			if !ok {
				return nil, ErrClosedConn
			}
			if ret.err == nil && p.expired(ret.conn) {
				_ = ret.conn.Close()
				return nil, ErrBadConn
			}
			if ret.conn == nil {
				return nil, ret.err
			}

			// 只分配无错误发生过的连接
			p.mu.Lock()
			err := ret.conn.GetLastErr()
			p.mu.Unlock()

			if err != nil {
				return nil, ErrBadConn
			}
			return ret.conn, nil
		}
	}

	// 如果不从空闲连接池中取，或者不限制最大连接数，或者当前连接数量未达到最大连接限制，则直接创建新连接即可
	// 连接数先自增，如果创建失败再恢复
	p.numOpened++
	p.mu.Unlock()

	cn, err := p.opt.dialer(ctx)
	if err != nil {
		p.mu.Lock()
		// 连接数恢复
		p.numOpened--
		// 检查空闲连接
		p.shouldOpenNewConnections()
		p.mu.Unlock()
		return nil, err
	}

	// 使用状态
	p.mu.Lock()
	cn.SetInUse(true)
	p.mu.Unlock()

	return cn, nil
}

// 关闭一个连接
func (p *Pool) closeConn(cn Connector) error {
	if p.opt.onClose != nil {
		p.opt.onClose(cn)
	}
	atomic.AddInt64(&p.numClosed, 1)
	return cn.Close()
}

// 获取请求队列 key
func (p *Pool) nextRequestKeyLocked() int64 {
	current := p.nextRequestKey
	p.nextRequestKey++
	return current
}

// 初始化时创建一定量的连接
//
// 不超过最大空闲数量
func (p *Pool) openInitialConnections() {
	maxIdle := p.maxIdle()
	initIdle := p.opt.initIdle
	if initIdle > maxIdle {
		initIdle = maxIdle
	}

	for initIdle > 0 {
		initIdle--
		if p.closed {
			return
		}
		p.openerCh <- struct{}{}
	}
}

// 检查是否需要创建新连接
//
// 创建请求的数量不能超过允许的最大数量
func (p *Pool) shouldOpenNewConnections() {
	numRequest := len(p.connRequests)
	if p.opt.maxOpen > 0 {
		// 如果队列中等待的请求数量 大于 连接池剩余空间，则已剩余空间为准
		numCanOpen := p.opt.maxOpen - p.numOpened
		if numRequest > numCanOpen {
			numRequest = numCanOpen
		}
	}
	// 创建满足请求量的连接
	for numRequest > 0 {
		// 已打开数量先自增（因为创建连接可能涉及异步操作），如果连接创建失败再还原即可
		p.numOpened++
		numRequest--
		if p.closed {
			return
		}
		p.openerCh <- struct{}{}
	}
}

// 创建连接
func (p *Pool) connectionOpener(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			//  context 超时
			return
		case <-p.openerCh:
			// 接收到该信号时，p.numOpened 已经自增
			// 创建新连接
			p.openNewConnection(ctx)
		}
	}
}

// 创建连接
func (p *Pool) openNewConnection(ctx context.Context) {
	// 创建新连接
	// 接收到 p.openerCh 信号时，p.numOpened 已经自增，所以如果这里创建连接失败，需要手动还原 p.numOpened
	cn, err := p.opt.dialer(ctx)

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		if err == nil {
			_ = p.closeConn(cn)
		}
		p.numOpened--
		return
	}

	if err != nil {
		p.numOpened--
		p.putConnLocked(nil, err)
		p.shouldOpenNewConnections()
		return
	}

	if !p.putConnLocked(cn, nil) {
		// 连接归还或重用不成功，直接舍弃即可
		p.numOpened--
		_ = p.closeConn(cn)
	}
}

// 归还连接
//
// 如果归还的是无效连接，就检查是否需要创建新连接
func (p *Pool) putConn(cn Connector, err error) {
	p.mu.Lock()
	if !cn.IsInUse() {
		// 连接未处于使用状态
		p.mu.Lock()
		return
	}

	cn.SetInUse(false)

	if err == ErrBadConn {
		// 当前归还的连接为错误连接，舍弃
		// 并检查空闲连接池
		p.shouldOpenNewConnections()
		p.mu.Unlock()
		_ = p.closeConn(cn)
		return
	}

	if p.opt.onPut != nil {
		p.opt.onPut(cn)
	}

	// 归还
	added := p.putConnLocked(cn, nil)
	p.mu.Unlock()
	if !added {
		// 归还失败，关闭当前连接
		_ = p.closeConn(cn)
	}
}

// 加锁，归还连接
//
// err == nil 时， cn 才有效
// return true  -> 归还成功
// return false -> 归还失败
func (p *Pool) putConnLocked(cn Connector, err error) bool {
	if p.closed {
		return false
	}
	if p.opt.maxOpen > 0 && p.numOpened > p.opt.maxOpen {
		// 当前连接数超过最大连接数限制，直接舍弃当前连接
		return false
	}

	// 先看请求队列中是否有等待请求
	// 有则取出一个请求，然后把当前连接分配给它
	// 否则将当前连接保存到空闲连接池中
	if l := len(p.connRequests); l > 0 {
		var (
			reqKey int64
			req    chan connRequest
		)
		for reqKey, req = range p.connRequests {
			// 只取一个请求
			break
		}
		// 然后把该请求从队列中移除
		delete(p.connRequests, reqKey)
		if err == nil {
			cn.SetInUse(true)
			if req != nil {
				req <- connRequest{
					conn: cn,
					err:  err,
				}
			}
			return true
		}
	} else if err == nil && !p.closed {
		// 如果连接池未满，则把连接归还到连接池
		if len(p.freeConn) < p.maxIdle() {
			p.freeConn = append(p.freeConn, cn)
			return true
		}
		p.maxIdleClosed++
	}
	return false
}

// 获取最大空闲连接数量
func (p *Pool) maxIdle() int {
	n := p.opt.maxIdle
	switch {
	case n < 0:
		return 0
	case n == 0:
		return defaultMaxIdleConn
	default:
		return n
	}
}

// 开始定时连接池清理操作
//
// 配置项 p.opt.maxLifetime > 0 时，该功能才生效
func (p *Pool) startCleanerLocked() {
	if p.opt.maxLifetime > 0 {
		// 最小 2s
		interval := p.opt.cleanerInterval
		if interval < 2*time.Second {
			interval = 2 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			<-ticker.C
			p.clean()
		}
	}
}

// 连接池清理
//
// 清理过时连接
func (p *Pool) clean() {
	p.mu.Lock()
	if p.closed || len(p.freeConn) == 0 {
		// 连接池已关闭，或者当前没有空闲连接
		p.mu.Unlock()
		return
	}

	var closing []Connector
	for i := 0; i < len(p.freeConn); i++ {
		cn := p.freeConn[i]
		if p.expired(cn) {
			// 该连接已过时，需移除
			closing = append(closing, cn)
			last := len(p.freeConn) - 1
			p.freeConn[i] = p.freeConn[last]
			p.freeConn[last] = nil
			p.freeConn = p.freeConn[:last]
			i--
		}
	}

	p.maxLifetimeClosed += int64(len(closing))
	p.mu.Unlock()

	// 关闭连接
	for _, cn := range closing {
		_ = p.closeConn(cn)
	}

	// 检查是否需要创建连接
	p.shouldOpenNewConnections()
}

// 判断某个连接是否已过时
//
// 配置项 p.opt.maxLifetime > 0 时，该功能才生效
func (p *Pool) expired(cn Connector) bool {
	return p.opt.maxLifetime > 0 && time.Since(cn.CreatedAt()) > p.opt.maxLifetime
}
