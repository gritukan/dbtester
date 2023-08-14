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
	"go.etcd.io/etcd/client/v3"

	"golang.org/x/net/context"
)

type request struct {
	etcdv3Op clientv3.Op
	zkOp     zkOp
	consulOp consulOp
}

// ReqHandler wraps request handler.
type ReqHandler func(ctx context.Context, req *request) error
