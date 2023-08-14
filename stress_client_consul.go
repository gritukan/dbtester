// Copyright 2017 CoreOS, Inc.
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

package dbtester

import (
	"go.uber.org/zap"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt/ytrpc"
	"golang.org/x/net/context"

	"go.ytsaurus.tech/yt/go/yt"
)

type consulOp struct {
	key       string
	value     []byte
	staleRead bool
}

func mustCreateConnsConsul(endpoints []string, total int64) []*yt.Client {
	css := make([]*yt.Client, total)
	for i := range css {
		c, err := ytrpc.NewClient(&yt.Config{
			RPCProxy:              "46.243.144.15:9013",
			DisableProxyDiscovery: true,
			Token:                 "password",
		})
		if err != nil {
			panic(err)
		}
		css[i] = &c
	}
	return css
}

func newPutConsul(conn *yt.Client) ReqHandler {
	return func(ctx context.Context, req *request) error {
		op := req.consulOp
		err := (*conn).SetNode(ctx, ypath.Path("//tmp/@"+op.key), op.value, nil)
		return err
	}
}

func newGetConsul(conn *yt.Client) ReqHandler {
	return func(ctx context.Context, req *request) error {
		var value any
		err := (*conn).GetNode(ctx, ypath.Path("//tmp/@"+req.consulOp.key), &value, nil)
		return err
	}
}

func getTotalKeysConsul(lg *zap.Logger, endpoints []string) map[string]int64 {
	rs := make(map[string]int64)
	for _, ep := range endpoints {
		rs[ep] = 0 // not supported in consul
	}
	return rs
}
