package guardian

import (
	"errors"
	"golang.org/x/sync/singleflight"
)

// Guardian cache guardian with singleflight support
type Guardian struct {
	g singleflight.Group
}

type GetAction func(key string) ([]byte, error)

type SetAction func(key string) (err error)

func New() *Guardian {
	return &Guardian{}
}

func (cm *Guardian) Get(key string, cmd GetAction) ([]byte, error) {
	v, err, _ := cm.g.Do(key, func() (interface{}, error) {
		b, err := cmd(key)
		return b, err
	})
	if err != nil {
		return nil, err
	}
	b, ok := v.([]byte)
	if !ok {
		return nil, errors.New("assert interface to []byte error")
	}

	return b, nil
}

func (cm *Guardian) Set(key string, cmd SetAction) error {
	_, err, _ := cm.g.Do(key, func() (interface{}, error) {
		return nil, cmd(key)
	})

	return err
}
