package sip

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"sip/header"
	"strconv"
	"strings"
	"sync"
)

type StartLineWriter interface {
	StartLineWrite(io.Writer) error
}

type Message interface {
	StartLineWriter

	GetSIPVersion() string
	SetSIPVersion(string) error
	GetHeader() Header
	SetHeader(Header)
	GetContentLength() int64
	SetContentLength(l int64)
	GetBody() io.Reader
	SetBody(io.Reader)
	Write(io.Writer) error
}

////////////////////////////////////////////////////////////////////////////////
type message struct {
	StartLineWriter

	sipVersion string
	header     Header

	/** Direct accessors for frequently accessed headers  **/
	via           []*header.Via
	from          *header.From
	to            *header.To
	cSeq          *header.CSeq
	callId        *header.CallID
	maxForwards   *header.MaxForwards
	contentLength *header.ContentLength

	//contentLength int64
	body io.Reader
}

func (this *message) GetSIPVersion() string {
	return this.sipVersion
}

func (this *message) SetSIPVersion(s string) error {
	if s != "SIP/2.0" {
		return errors.New("Wrong SIP Version")
	} else {
		this.sipVersion = s
		return nil
	}
}

func (this *message) GetHeader() Header {
	return this.header
}

func (this *message) SetHeader(header Header) {
	this.header = header
}

func (this *message) GetContentLength() int64 {
	if this.contentLength != nil {
		return int64(this.contentLength.GetContentLength())
	} else {
		return 0
	}
}

func (this *message) SetContentLength(l int64) {
	if this.contentLength == nil {
		this.contentLength = header.NewContentLength()
	}
	this.contentLength.SetContentLength(int(l))
}

func (this *message) GetBody() io.Reader {
	return this.body
}

func (this *message) SetBody(body io.Reader) {
	this.body = body
}

// Headers that Request.Write handles itself and should be skipped.
var reqWriteExcludeHeader = map[string]bool{
	"Content-Length": true,
}

//  Start-Line
//	Header
//	ContentLength
//	Body
func (this *message) Write(w io.Writer) (err error) {
	var bw *bufio.Writer
	if _, ok := w.(io.ByteWriter); !ok {
		bw = bufio.NewWriter(w)
		w = bw
	}

	if err = this.StartLineWriter.StartLineWrite(w); err != nil {
		return err
	}

	if err = this.header.WriteSubset(w, reqWriteExcludeHeader); err != nil {
		return err
	}

	if _, err = fmt.Fprintf(w, "%s: %d\r\n", "Content-Length", this.GetContentLength()); err != nil {
		return err
	}

	if _, err = io.WriteString(w, "\r\n"); err != nil {
		return err
	}

	// Write body
	if this.body != nil {
		if _, err = io.Copy(w, io.LimitReader(this.body, this.GetContentLength())); err != nil {
			return err
		}
	}

	return nil
}

// ReadMessage reads and parses an incoming message from b.
func ReadMessage(b *bufio.Reader) (msg Message, err error) {
	tp := newTextprotoReader(b)

	// First line: INVITE sip:bob@biloxi.com SIP/2.0 or SIP/2.0 180 Ringing
	var s string
	if s, err = tp.ReadLine(); err != nil {
		return nil, err
	}
	defer func() {
		putTextprotoReader(tp)
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
	}()

	s1 := strings.Index(s, " ")
	s2 := strings.Index(s[s1+1:], " ")
	if s1 < 0 || s2 < 0 {
		return nil, fmt.Errorf("malformed SIP request %s", s)
	}
	s2 += s1 + 1

	if strings.TrimSpace(s[:s1]) == "SIP/2.0" {
		var statusCode int
		if statusCode, err = strconv.Atoi(s[s1+1 : s2]); err != nil {
			return nil, fmt.Errorf("malformed SIP status code %s", s[s1+1:s2])
		}
		sipVersion, reasonPhrase := s[:s1], s[s2+1:]
		if _, _, ok := ParseSIPVersion(sipVersion); !ok {
			return nil, fmt.Errorf("malformed SIP version", sipVersion)
		}
		msg = NewResponse(statusCode, reasonPhrase, nil)
	} else {
		method, requestURI, sipVersion := s[:s1], s[s1+1:s2], s[s2+1:]
		if _, _, ok := ParseSIPVersion(sipVersion); !ok {
			return nil, fmt.Errorf("malformed SIP version", sipVersion)
		}
		msg = NewRequest(method, requestURI, nil)
	}

	////////////////////////////////////////////////////////////////////////////
	// Subsequent lines: Key: value.
	mimeHeader, err := tp.ReadMIMEHeader()
	if err != nil {
		return nil, err
	}
	msg.SetHeader(Header(mimeHeader))

	////////////////////////////////////////////////////////////////////////////

	contentLens := msg.GetHeader()["Content-Length"]
	if len(contentLens) > 1 { // harden against SIP request smuggling. See RFC 7230.
		return nil, errors.New("http: message cannot contain multiple Content-Length headers")
	}

	// Logic based on Content-Length
	var cl string
	if len(contentLens) == 1 {
		cl = strings.TrimSpace(contentLens[0])
	}
	if cl != "" {
		n, err := parseContentLength(cl)
		if err != nil {
			return nil, err
		}
		msg.SetContentLength(n)
	} else {
		msg.GetHeader().Del("Content-Length")
		msg.SetContentLength(0)
	}

	////////////////////////////////////////////////////////////////////////////

	if msg.GetContentLength() > 0 {
		msg.SetBody(io.LimitReader(b, int64(msg.GetContentLength())))
	} else {
		msg.SetBody(nil)
	}

	return msg, nil
}

// parseContentLength trims whitespace from s and returns -1 if no value
// is set, or the value if it's >= 0.
func parseContentLength(cl string) (int64, error) {
	cl = strings.TrimSpace(cl)
	if cl == "" {
		return -1, nil
	}
	n, err := strconv.ParseInt(cl, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("bad Content-Length %d", cl)
	}
	return n, nil

}

var textprotoReaderPool sync.Pool

func newTextprotoReader(br *bufio.Reader) *textproto.Reader {
	if v := textprotoReaderPool.Get(); v != nil {
		tr := v.(*textproto.Reader)
		tr.R = br
		return tr
	}
	return textproto.NewReader(br)
}

func putTextprotoReader(r *textproto.Reader) {
	r.R = nil
	textprotoReaderPool.Put(r)
}

// ParseSIPVersion parses a SIP version string.
// "SIP/2.0" returns (2, 0, true).
func ParseSIPVersion(vers string) (major, minor int, ok bool) {
	const Big = 1000000 // arbitrary upper bound
	switch vers {
	case "SIP/2.0":
		return 2, 0, true
	}
	if !strings.HasPrefix(vers, "SIP/") {
		return 0, 0, false
	}
	dot := strings.Index(vers, ".")
	if dot < 0 {
		return 0, 0, false
	}
	major, err := strconv.Atoi(vers[4:dot])
	if err != nil || major < 0 || major > Big {
		return 0, 0, false
	}
	minor, err = strconv.Atoi(vers[dot+1:])
	if err != nil || minor < 0 || minor > Big {
		return 0, 0, false
	}
	return major, minor, true
}
