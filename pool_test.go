/*
 * Copyright 2013 Nan Deng
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package connpool

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

type fakeConn struct {
	err error
}

func (self *fakeConn) Read(b []byte) (n int, err error) {
	if self.err != nil {
		return 0, self.err
	}
	return len(b), nil
}

func (self *fakeConn) Write(b []byte) (n int, err error) {
	if self.err != nil {
		return 0, self.err
	}
	return len(b), nil
}

func (self *fakeConn) Close() error {
	return nil
}

func (self *fakeConn) LocalAddr() net.Addr {
	return nil
}
func (self *fakeConn) RemoteAddr() net.Addr {
	return nil
}

func (self *fakeConn) SetDeadline(t time.Time) error {
	return nil
}

func (self *fakeConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (self *fakeConn) SetWriteDeadline(t time.Time) error {
	return nil
}

type fakeConnManager struct {
	err     error
	connErr error
}

func (self *fakeConnManager) NewConn() (conn net.Conn, err error) {
	if self.err != nil {
		return nil, self.err
	}
	return &fakeConn{self.connErr}, nil
}

func (self *fakeConnManager) InitConn(conn net.Conn) error {
	return self.err
}

func TestPushPopIdleWithinRange(t *testing.T) {
	N := 10
	mid := 5
	manager := &fakeConnManager{nil, nil}
	pool := NewPool(N, N, manager)
	connList := make([]*pooledConn, 0, N)
	for i := 0; i < N; i++ {
		conn, _ := pool.createConn()
		if conn == nil {
			t.Errorf("Got nil conn")
			pool.debug(1)
			return
		}
		connList = append(connList, conn)
	}

	if pool.nrActiveConn != N {
		t.Errorf("#. Active Connections is %v (should be %v)",
			pool.nrActiveConn, N)
		return
	}
	if len(pool.idle) != 0 {
		t.Errorf("idle list is not empty!")
		return
	}

	for i := 0; i < mid; i++ {
		pool.pushIdle(connList[i])
	}
	if len(pool.idle) != mid {
		t.Errorf("#. Idle Connections is %v (should be %v)",
			len(pool.idle), mid)
		return
	}
	if pool.nrActiveConn != N-mid {
		t.Errorf("#. Active Connections is %v (should be %v)",
			pool.nrActiveConn, N-mid)
		return
	}
	for i := mid; i < N; i++ {
		pool.popIdle()
	}

	if pool.nrActiveConn != N {
		t.Errorf("#. Active Connections is %v (should be %v)",
			pool.nrActiveConn, N)
		return
	}
	if len(pool.idle) != 0 {
		t.Errorf("idle list is not empty!")
		return
	}
}

func TestPushPopIdleOutOfRange(t *testing.T) {
	N := 10
	max := 8
	manager := &fakeConnManager{nil, nil}
	pool := NewPool(max, max, manager)
	defer pool.Close()
	connList := make([]*pooledConn, 0, N)
	for i := 0; i < N; i++ {
		conn, _ := pool.createConn()
		connList = append(connList, conn)
	}

	if pool.nrActiveConn != max {
		t.Errorf("#. Active Connections is %v (should be %v)",
			pool.nrActiveConn, max)
		return
	}
	if len(pool.idle) != 0 {
		t.Errorf("idle list is not empty!")
		return
	}

	for i := 0; i < N; i++ {
		conn := connList[i]
		if conn == nil {
			c := &fakeConn{nil}
			conn = &pooledConn{c, nil, nil}
		}
		pool.pushIdle(conn)
	}

	if pool.nrActiveConn != 0 {
		t.Errorf("There should be no active connections")
	}
	if len(pool.idle) != max {
		t.Errorf("idle list length is %v, should be %v!",
			len(pool.idle), max)
		return
	}
}

func integrityTest(pool *Pool, maxIdle, maxConn int) error {
	if len(pool.idle) > maxIdle {
		return fmt.Errorf("len(idle) > maxIdle: %v > %v", len(pool.idle), maxIdle)
	}
	if len(pool.idle)+pool.nrActiveConn > maxConn {
		return fmt.Errorf("len(idle)+nrActiveConn > maxCOnn: %v+%v > %v", len(pool.idle), pool.nrActiveConn, maxConn)
	}
	return nil
}

func TestGetConnWithinRange(t *testing.T) {
	N := 10
	max := 10
	manager := &fakeConnManager{nil, nil}
	pool := NewPool(max, max, manager)
	defer pool.Close()
	connList := make([]net.Conn, 0, N)
	for i := 0; i < N; i++ {
		conn, err := pool.Get()
		if err != nil {
			t.Errorf("Error: %v", err)
			return
		}
		connList = append(connList, conn)
	}
	if pool.nrActiveConn != max {
		t.Errorf("#. Active Connections is %v (should be %v)",
			pool.nrActiveConn, max)
		return
	}
	err := integrityTest(pool, N, N)
	if err != nil {
		t.Errorf("Pool is corrupted: %v\n", err)
		return
	}

	for _, conn := range connList {
		conn.Close()
	}

	if len(pool.idle) != max {
		t.Errorf("idle list length is %v; should be %v", len(pool.idle), max)
		return
	}
	err = integrityTest(pool, N, N)
	if err != nil {
		t.Errorf("Pool is corrupted: %v\n", err)
		return
	}
}

func TestGetConnOutOfRange(t *testing.T) {
	N := 5
	max := 3
	manager := &fakeConnManager{nil, nil}
	pool := NewPool(max, max, manager)
	defer pool.Close()
	connList := make([]net.Conn, 0, N)
	for i := 0; i < max; i++ {
		conn, err := pool.Get()
		if err != nil {
			t.Errorf("Error: %v", err)
			return
		}
		if conn == nil {
			t.Errorf("nil conn")
			return
		}
		connList = append(connList, conn)
	}
	if pool.nrActiveConn != max {
		t.Errorf("#. Active Connections is %v (should be %v)",
			pool.nrActiveConn, max)
		return
	}

	err := integrityTest(pool, N, N)
	if err != nil {
		t.Errorf("Pool is corrupted: %v\n", err)
		return
	}

	wg := new(sync.WaitGroup)
	wg.Add(N - max)

	for i := max; i < N; i++ {
		go func() {
			conn, err := pool.Get()
			if err != nil {
				t.Errorf("Error: %v", err)
				return
			}
			if conn == nil {
				t.Errorf("nil conn")
				return
			}
			wg.Done()
		}()
	}

	for _, conn := range connList {
		conn.Close()
	}
	wg.Wait()
}

func TestGetWithError(t *testing.T) {
	max := 8
	errSomeError := fmt.Errorf("shit happens")
	manager := &fakeConnManager{errSomeError, nil}
	pool := NewPool(max, max, manager)
	defer pool.Close()

	_, err := pool.Get()
	if err != errSomeError {
		fmt.Errorf("shit really happens")
	}
}

func TestGetFromIdleList(t *testing.T) {
	max := 1
	manager := &fakeConnManager{nil, nil}
	pool := NewPool(max, max, manager)
	defer pool.Close()

	conn, err := pool.Get()
	if err != nil {
		t.Errorf("Error: %v", err)
		return
	}

	if conn == nil {
		t.Errorf("nil conn")
		return
	}

	wg := new(sync.WaitGroup)
	wg.Add(1)

	go func() {
		defer wg.Done()
		c, err := pool.Get()
		if err != nil {
			t.Errorf("Error: %v", err)
			return
		}
		if c == nil {
			t.Errorf("nil conn")
		}
	}()

	// Wait for a while, so that the
	// other goroutine have to wait on Get()
	// (let its request enqueue)
	time.Sleep(1 * time.Second)
	conn.Close()
	wg.Wait()
}
