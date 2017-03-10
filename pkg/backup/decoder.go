// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package backup

import (
	"encoding/binary"
	"io"

	"github.com/CodisLabs/codis/pkg/utils/bufio2"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/golang/protobuf/proto"
)

var (
	ErrBadRes = errors.New("bad repsonse length")
)

type Decoder struct {
	br *bufio2.Reader

	Err error
}

var ErrFailedDecoder = errors.New("use of failed decoder")

func NewDecoder(r io.Reader) *Decoder {
	return NewDecoderBuffer(bufio2.NewReaderSize(r, 8192))
}

func NewDecoderSize(r io.Reader, size int) *Decoder {
	return NewDecoderBuffer(bufio2.NewReaderSize(r, size))
}

func NewDecoderBuffer(br *bufio2.Reader) *Decoder {
	return &Decoder{br: br}
}

func (d *Decoder) DecodeReq() (*WriteRequest, error) {
	if d.Err != nil {
		return nil, errors.Trace(ErrFailedDecoder)
	}
	r, err := d.decodeReq()
	if err != nil {
		d.Err = err
	}
	return r, d.Err
}

func (d *Decoder) DecodeRes() (*WriteResponse, error) {
	if d.Err != nil {
		return nil, errors.Trace(ErrFailedDecoder)
	}
	r, err := d.decodeRes()
	if err != nil {
		d.Err = err
	}
	return r, d.Err
}

func (d *Decoder) decodeReq() (*WriteRequest, error) {
	b, err := d.br.ReadFull(HeaderSize)
	if err != nil {
		return nil, errors.Trace(err)
	}
	length := binary.BigEndian.Uint32(b)
	if length <= HeaderSize {
		return nil, ErrBadRes
	}
	log.Warnf("recv packet length: %d\n", length)
	b, err = d.br.ReadFull(int(length) - HeaderSize)
	if err != nil {
		return nil, errors.Trace(err)
	}
	r := &WriteRequest{}
	err = proto.Unmarshal(b, r)
	return r, err
}

func (d *Decoder) decodeRes() (*WriteResponse, error) {
	b, err := d.br.ReadFull(HeaderSize)
	if err != nil {
		return nil, errors.Trace(err)
	}
	length := binary.BigEndian.Uint32(b)
	if length <= HeaderSize {
		return nil, ErrBadRes
	}
	log.Warnf("recv packet length: %d\n", length)
	b, err = d.br.ReadFull(int(length) - HeaderSize)
	if err != nil {
		return nil, errors.Trace(err)
	}
	r := &WriteResponse{}
	err = proto.Unmarshal(b, r)
	return r, err
}
