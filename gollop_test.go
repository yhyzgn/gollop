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
// time   : 2020-04-02 8:19 下午
// version: 1.0.0
// desc   : 

package gollop

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"testing"
	"time"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var rd *rand.Rand

func init() {
	rd = rand.New(rand.NewSource(now().Unix()))
}

// randString 生成随机字符串
func randString(len int) string {
	bytes := make([]byte, len)
	for i := 0; i < len; i++ {
		b := rd.Intn(52)
		bytes[i] = letterBytes[b]
	}
	return string(bytes)
}

type testConn struct {
	sync.Mutex
	code      string
	conn      net.Conn
	createdAt time.Time
	inUse     bool
	lastErr   error
}

func (t *testConn) CreatedAt() time.Time {
	return t.createdAt
}

func (t *testConn) GetLocker() sync.Locker {
	return t
}

func (t *testConn) SetInUse(inUse bool) {
	t.inUse = inUse
}

func (t *testConn) IsInUse() bool {
	return t.inUse
}

func (t *testConn) GetLastErr() error {
	return t.lastErr
}

func (t *testConn) Close() error {
	return t.conn.Close()
}

type client struct {
	pl *Pool
}

func newClient() *client {
	return &client{
		pl: New(
			MaxIdle(2),
			MaxOpen(4),
			MaxLifetime(30*time.Second),
			Dialer(func(ctx context.Context) (connector Connector, err error) {
				dialer := net.Dialer{
					Timeout:   6 * time.Second,
					KeepAlive: 6 * time.Second,
				}
				cn, err := dialer.Dial("tcp", "localhost:6379")
				if err != nil {
					return nil, err
				}
				return &testConn{
					code:      randString(16),
					conn:      cn,
					createdAt: now(),
				}, nil
			}),
			OnPut(func(cn Connector) {
				fmt.Println("归还连接：" + (cn.(*testConn)).code)
			}),
			OnClose(func(cn Connector) {
				fmt.Println("正在关闭连接：" + (cn.(*testConn)).code)
			}),
		),
	}
}

func (c *client) get() *testConn {
	cn, err := c.pl.Get()
	if err != nil {
		fmt.Println("获取连接失败 - ", err)
		return nil
	}
	return cn.(*testConn)
}

func (c *client) release(cn *testConn) {
	if cn != nil {
		c.pl.Put(cn)
	}
}

func TestNew(t *testing.T) {
	c := newClient()
	// 首次获取连接，用完及时归还
	cn := c.get()
	fmt.Println(cn)
	c.release(cn)

	// 协程，模拟异步获取连接，并归还
	go func() {
		cn := c.get()
		fmt.Println(cn)
		time.Sleep(3 * time.Second)
		c.release(cn)
	}()

	cn = c.get()
	fmt.Println(cn)

	cn = c.get()
	fmt.Println(cn)
	a := cn

	cn = c.get()
	fmt.Println(cn)
	c.release(cn)

	cn = c.get()
	fmt.Println(cn)

	fmt.Println(c.get())

	time.Sleep(10 * time.Second)
	c.release(cn)

	fmt.Println(c.pl.Status().String())

	c.release(a)
	time.Sleep(5 * time.Second)
	fmt.Println(c.get())

	time.Sleep(time.Hour)
}
