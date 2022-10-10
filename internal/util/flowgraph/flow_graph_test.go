// Licensed to the LF AI & Data foundation under one
// or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership. The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License. You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package flowgraph

import (
	"context"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Flow graph basic example: count `c = pow(a) + 2`
// nodeA: receive input value a from input channel
// nodeB: count b = pow(a, 2)
// nodeD: count c = b + 2

type nodeA struct {
	BaseNode
	inputChan chan float64
	a         float64
}

type nodeB struct {
	BaseNode
	b float64
}

type nodeC struct {
	BaseNode
	d          float64
	outputChan chan float64
}

type numMsg struct {
	num float64
}

func (m *numMsg) TimeTick() Timestamp {
	return Timestamp(0)
}

func (n *nodeA) Name() string {
	return "NodeA"
}

func (n *nodeA) Operate(in []Msg) []Msg {
	// ignore `in` because nodeA doesn't have any upstream node.
	a := <-n.inputChan
	var res Msg = &numMsg{
		num: a,
	}
	return []Msg{res}
}

func (n *nodeB) Name() string {
	return "NodeB"
}

func (n *nodeB) Operate(in []Msg) []Msg {
	a, ok := in[0].(*numMsg)
	if !ok {
		return nil
	}
	b := math.Pow(a.num, 2)
	var res Msg = &numMsg{
		num: b,
	}
	return []Msg{res}
}

func (n *nodeC) Name() string {
	return "NodeC"
}

func (n *nodeC) Operate(in []Msg) []Msg {
	b, ok := in[0].(*numMsg)
	if !ok {
		return nil
	}
	c := b.num + 2
	n.outputChan <- c
	// return nil because nodeD doesn't have any downstream node.
	return nil
}

func createExampleFlowGraph() (*TimeTickedFlowGraph, chan float64, chan float64, context.CancelFunc, error) {
	const MaxQueueLength = 1024

	ctx, cancel := context.WithCancel(context.Background())
	inputChan := make(chan float64, MaxQueueLength)
	outputChan := make(chan float64, MaxQueueLength)

	fg := NewTimeTickedFlowGraph(ctx)

	var a Node = &nodeA{
		BaseNode: BaseNode{
			maxQueueLength: MaxQueueLength,
		},
		inputChan: inputChan,
	}
	var b Node = &nodeB{
		BaseNode: BaseNode{
			maxQueueLength: MaxQueueLength,
		},
	}
	var c Node = &nodeC{
		BaseNode: BaseNode{
			maxQueueLength: MaxQueueLength,
		},
		outputChan: outputChan,
	}

	fg.AddNode(a)
	fg.AddNode(b)
	fg.AddNode(c)

	var err = fg.SetEdges(a.Name(),
		[]string{b.Name()},
	)
	if err != nil {
		return nil, nil, nil, cancel, err
	}

	err = fg.SetEdges(b.Name(),
		[]string{c.Name()},
	)
	if err != nil {
		return nil, nil, nil, cancel, err
	}

	err = fg.SetEdges(c.Name(),
		[]string{},
	)
	if err != nil {
		return nil, nil, nil, cancel, err
	}

	return fg, inputChan, outputChan, cancel, nil
}

func TestTimeTickedFlowGraph_AddNode(t *testing.T) {
	const MaxQueueLength = 1024
	inputChan := make(chan float64, MaxQueueLength)

	fg := NewTimeTickedFlowGraph(context.TODO())

	var a Node = &nodeA{
		BaseNode: BaseNode{
			maxQueueLength: MaxQueueLength,
		},
		inputChan: inputChan,
	}
	var b Node = &nodeB{
		BaseNode: BaseNode{
			maxQueueLength: MaxQueueLength,
		},
	}

	fg.AddNode(a)
	assert.Equal(t, len(fg.nodeCtx), 1)
	fg.AddNode(b)
	assert.Equal(t, len(fg.nodeCtx), 2)
}

func TestTimeTickedFlowGraph_Start(t *testing.T) {
	fg, inputChan, outputChan, cancel, err := createExampleFlowGraph()
	assert.NoError(t, err)
	defer cancel()
	fg.Start()

	// input
	go func() {
		for i := 0; i < 10; i++ {
			a := float64(rand.Int())
			inputChan <- a

			// output check
			d := <-outputChan
			res := math.Pow(a, 2) + 2
			assert.Equal(t, d, res)
		}
	}()
	time.Sleep(50 * time.Millisecond)
}

func TestTimeTickedFlowGraph_Close(t *testing.T) {
	fg, _, _, cancel, err := createExampleFlowGraph()
	assert.NoError(t, err)
	defer cancel()
	fg.Close()
}
