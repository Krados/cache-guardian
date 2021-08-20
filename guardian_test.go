package guardian

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

type FakeAPICache struct {
	OpsGet int32
	OpsSet int32
	M      map[string][]byte
	sync.Mutex
}

func (f *FakeAPICache) Set(key string, value []byte, d time.Duration) error {
	f.Lock()
	defer f.Unlock()
	// use it if needed
	_ = d

	// simulate action latency
	time.Sleep(time.Second * 3)
	f.OpsSet += 1
	f.M[key] = value

	return nil
}

func (f *FakeAPICache) Get(key string) ([]byte, error) {
	f.Lock()
	defer f.Unlock()
	// simulate action latency
	time.Sleep(time.Second * 3)
	val, ok := f.M[key]
	if !ok {
		return nil, errors.New(fmt.Sprintf("key %v does not exists", key))
	}
	f.OpsGet += 1
	return val, nil
}

func (f *FakeAPICache) GetOpsGet() int32 {
	return f.OpsGet
}

func (f *FakeAPICache) GetOpsSet() int32 {
	return f.OpsSet
}

func TestGuardian_Get(t *testing.T) {
	fc := &FakeAPICache{}
	key := "testKey"
	g := New()

	dataByte, err := g.Get(key, func(key string) ([]byte, error) {
		return fc.Get(key)
	})
	if err != nil {
		if bytes.Equal([]byte(key), dataByte) {
			t.Errorf("want %v buy got %v", []byte(key), dataByte)
		}
	}
}

func TestGuardian_Set(t *testing.T) {
	fc := &FakeAPICache{M: make(map[string][]byte)}
	key := "testKey"
	cm := New()
	err := cm.Set(key, func(key string) (err error) {
		return fc.Set(key, []byte(key), time.Second)
	})
	if err != nil {
		t.Errorf("cm.Set err %v", err)
	}

	dataByte, err := cm.Get(key, func(key string) ([]byte, error) {
		return fc.Get(key)
	})
	if err != nil {
		t.Errorf("cm.Get err %v", err)
	}

	if !bytes.Equal([]byte(key), dataByte) {
		t.Errorf("want %v buy got %v", []byte(key), dataByte)
	}
}

func TestGuardian_ConcurrentlyGet(t *testing.T) {
	haltSignal := make(chan struct{})
	fc := &FakeAPICache{M: make(map[string][]byte)}
	key := "testKey"
	cm := New()
	err := cm.Set(key, func(key string) (err error) {
		return fc.Set(key, []byte(key), time.Second)
	})
	if err != nil {
		t.Errorf("cm.Set err %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func(innerCM *Guardian, innerHalt chan struct{}) {
		<-innerHalt
		_, err := innerCM.Get(key, func(key string) ([]byte, error) {
			return fc.Get(key)
		})
		if err != nil {
			t.Errorf("cm.Get err %v", err)
		}
		defer wg.Done()
	}(cm, haltSignal)
	go func(innerCM *Guardian, innerHalt chan struct{}) {
		<-innerHalt
		_, err := innerCM.Get(key, func(key string) ([]byte, error) {
			return fc.Get(key)
		})
		if err != nil {
			t.Errorf("cm.Get err %v", err)
		}
		defer wg.Done()
	}(cm, haltSignal)

	// sleep for a while til all goroutines are ready
	time.Sleep(time.Second * 2)
	close(haltSignal)

	// wait goroutines finishes their jobs
	wg.Wait()

	var wantOps int32 = 1
	ops := fc.GetOpsGet()
	if wantOps != ops {
		t.Errorf("want %v but got %v", wantOps, ops)
	}
}

func TestGuardian_ConcurrentlySet(t *testing.T) {
	haltSignal := make(chan struct{})
	fc := &FakeAPICache{M: make(map[string][]byte)}
	key := "testKey"
	cm := New()
	setFunc := func(key string) (err error) {
		return fc.Set(key, []byte(key), time.Second)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func(innerCM *Guardian, innerHalt chan struct{}) {
		<-innerHalt
		err := innerCM.Set(key, setFunc)
		if err != nil {
			t.Errorf("cm.Get err %v", err)
		}
		defer wg.Done()
	}(cm, haltSignal)
	go func(innerCM *Guardian, innerHalt chan struct{}) {
		<-innerHalt
		err := innerCM.Set(key, setFunc)
		if err != nil {
			t.Errorf("cm.Get err %v", err)
		}
		defer wg.Done()
	}(cm, haltSignal)

	// sleep for a while til all goroutines are ready
	time.Sleep(time.Second * 2)
	close(haltSignal)

	// wait goroutines finishes their jobs
	wg.Wait()

	var wantOps int32 = 1
	ops := fc.GetOpsSet()
	if wantOps != ops {
		t.Errorf("want %v but got %v", wantOps, ops)
	}
}
