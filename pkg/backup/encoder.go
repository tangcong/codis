// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package backup

import (
	"encoding/binary"
	"github.com/CodisLabs/codis/pkg/utils/bufio2"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/golang/protobuf/proto"
	"io"
)

type Encoder struct {
	bw *bufio2.Writer

	Err error
}

var ErrFailedEncoder = errors.New("use of failed encoder")

func NewEncoder(w io.Writer) *Encoder {
	return NewEncoderBuffer(bufio2.NewWriterSize(w, 8192))
}

func NewEncoderSize(w io.Writer, size int) *Encoder {
	return NewEncoderBuffer(bufio2.NewWriterSize(w, size))
}

func NewEncoderBuffer(bw *bufio2.Writer) *Encoder {
	return &Encoder{bw: bw}
}

func (e *Encoder) EncodeReq(r *WriteRequest, flush bool) error {
	if e.Err != nil {
		return errors.Trace(ErrFailedEncoder)
	}
	if err := e.encodeReq(r); err != nil {
		e.Err = err
	} else if flush {
		e.Err = errors.Trace(e.bw.Flush())
	}
	return e.Err
}

func (e *Encoder) EncodeRes(r *WriteResponse, flush bool) error {
	if e.Err != nil {
		return errors.Trace(ErrFailedEncoder)
	}
	if err := e.encodeRes(r); err != nil {
		e.Err = err
	} else if flush {
		e.Err = errors.Trace(e.bw.Flush())
	}
	return e.Err
}

func (e *Encoder) Flush() error {
	if e.Err != nil {
		return errors.Trace(ErrFailedEncoder)
	}
	if err := e.bw.Flush(); err != nil {
		e.Err = errors.Trace(err)
	}
	return e.Err
}

func Encode(w io.Writer, r *WriteRequest) error {
	return NewEncoder(w).EncodeReq(r, true)
}

func (e *Encoder) encodeReq(r *WriteRequest) error {
	var (
		b      []byte
		length uint
		err    error
	)
	b, err = proto.Marshal(r)
	length = HeaderSize + uint(len(b))
	var buf = make([]byte, length)
	binary.BigEndian.PutUint32(buf[0:], uint32(length))
	copy(buf[HeaderSize:], b)
	if _, err := e.bw.Write(buf); err != nil {
		return errors.Trace(err)
	}
	return err
}

func (e *Encoder) encodeRes(r *WriteResponse) error {
	var (
		b      []byte
		length uint
		err    error
	)
	b, err = proto.Marshal(r)
	length = HeaderSize + uint(len(b))
	var buf = make([]byte, length)
	binary.BigEndian.PutUint32(buf[0:], uint32(length))
	copy(buf[HeaderSize:], b)
	if _, err := e.bw.Write(buf); err != nil {
		return errors.Trace(err)
	}
	return err
}
