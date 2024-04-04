package entity

import (
	"context"
	"errors"
)

const (
	deviceTUN = iota + 1
	deviceTAP

	DeviceTypeTUN DeviceType = "tun"
	DeviceTypeTAP DeviceType = "tap"
)

type DeviceType string

func (d DeviceType) Int() int {
	switch d {
	case DeviceTypeTUN:
		return deviceTUN
	case DeviceTypeTAP:
		return deviceTAP
	default:
		return 0
	}
}

type InterfaceReceiver func(*Message) error

type Interface struct {
	Type     DeviceType
	IP       IfIP
	Handler  InterfaceHandler
	Receiver chan *Message
	Cancel   context.CancelFunc
}

func (i Interface) Name() (string, error) {
	if i.Handler == nil {
		return "", errors.New("handler is nil")
	}
	return i.Handler.Name()
}

func (i Interface) Close() error {
	if i.Handler == nil {
		return ErrInterfaceNotExists
	}
	i.Cancel()
	err := i.Handler.Close()
	return err
}

type InterfaceHandler interface {
	Name() (string, error)
	Close() error
	Read(p []byte) (n int, err error)
	Write(p []byte) (n int, err error)
}

type InterfaceAdapter interface {
	Create(*Interface) (*Interface, error)
	Write(*Interface, interface{}) error
	Close(*Interface) error
	SendLog(*Message, string)
	ReceiveLog(*Message)
}
