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
// time   : 2020-04-01 10:36 下午
// version: 1.0.0
// desc   : 连接池配置项

package gollop

import (
	"context"
	"runtime"
	"time"
)

type Options func(*options)

type options struct {
	waitTimeout     time.Duration                                // 获取连接时的等待超时时长，默认 6s
	maxIdle         int                                          // 最大空闲数量，默认 defaultMaxIdleConn = 4 * runtime.NumCPU()
	initIdle        int                                          // 初始化时需创建的空闲数量，<= maxIdle，默认 runtime.NumCPU()
	maxOpen         int                                          // 最大连接数量，0 表示不限制，默认 10 * runtime.NumCPU()
	maxLifetime     time.Duration                                // 连接生命时长，<= 0 时表示永不过期
	cleanerInterval time.Duration                                // 连接清理器执行间隔时长，>= 2s 默认 2s
	dialer          func(ctx context.Context) (Connector, error) // 具体连接操作实现
	onPut           func(connector Connector)                    // 连接回收回调
	onClose         func(connector Connector)                    // 连接关闭回调
}

// 默认配置
func defOptions() *options {
	return &options{
		waitTimeout: 6 * time.Second,
		maxIdle:     defaultMaxIdleConn,
		initIdle:    runtime.NumCPU(),
		maxOpen:     10 * runtime.NumCPU(),
	}
}

// 设置连接等待超时时长
//
// 默认 6s
func WaitTimeout(timeout time.Duration) Options {
	return func(o *options) {
		o.waitTimeout = timeout
	}
}

// 设置最大空闲连接数量
//
// 默认 defaultMaxIdleCon = 4 * runtime.NumCPU()
func MaxIdle(maxIdle int) Options {
	return func(o *options) {
		o.maxIdle = maxIdle
	}
}

// 初始化时需创建的空闲数量
//
// <= maxIdle，默认 runtime.NumCPU()
func InitIdle(initIdle int) Options {
	return func(o *options) {
		o.initIdle = initIdle
	}
}

// 设置最大连接数量
//
// 0 表示不限制，默认 10 * runtime.NumCPU()
func MaxOpen(maxOpen int) Options {
	return func(o *options) {
		o.maxOpen = maxOpen
	}
}

// 设置连接生命时长
//
// <= 0 表示永不过期，默认 0
func MaxLifetime(maxLifetime time.Duration) Options {
	return func(o *options) {
		o.maxLifetime = maxLifetime
	}
}

// 连接清理器执行间隔时长
//
// >= 2s 默认 2s
func CleanerInterval(cleanerInterval time.Duration) Options {
	return func(o *options) {
		o.cleanerInterval = cleanerInterval
	}
}

// 具体连接操作实现
func Dialer(dialer func(ctx context.Context) (Connector, error)) Options {
	return func(o *options) {
		o.dialer = dialer
	}
}

// 归还连接回调
func OnPut(onPut func(cn Connector)) Options {
	return func(o *options) {
		o.onPut = onPut
	}
}

// 关闭连接回调
func OnClose(onClose func(cn Connector)) Options {
	return func(o *options) {
		o.onClose = onClose
	}
}
