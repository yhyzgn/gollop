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
// time   : 2020-04-03 14:41
// version: 1.0.0
// desc   : 连接池状态

package gollop

import (
	"encoding/json"
	"time"
)

// 连接池状态
type Status struct {
	PoolSize          int           // 当前连接数量
	MaxPoolSize       int           // 最大连接数量
	IdleSize          int           // 当前空闲连接数量
	MaxIdleSize       int           // 最大空闲连接数量
	InUseSize         int           // 正在使用的连接数量
	WaitCount         int64         // 等待连接的请求数量
	WaitDuration      time.Duration // 总等待时长
	MaxIdleClosed     int64         // 归还连接时，关闭超出连接数量
	MaxLifetimeClosed int64         // 空闲超时需要关闭的连接数量
	ClosedCount       int64         // 已关闭连接数量
}

func (s Status) String() string {
	data, err := json.Marshal(s)
	if err != nil {
		return err.Error()
	}
	return string(data)
}
