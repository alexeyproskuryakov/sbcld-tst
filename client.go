package main
import (
	"net/http"
	"io"
	"fmt"
	"context"
	"crypto/tls"
	"strings"
	"errors"
	"strconv"
	"bytes"
	"time"
)


type RangedRetrieableTransport struct {
    http.RoundTripper 
	ctx context.Context
}

func NewRangedRetrieableTransport(ctx context.Context, upstream *http.Transport) *RangedRetrieableTransport {
    upstream.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
    result := &RangedRetrieableTransport{upstream, ctx}
	return result
}

func processResponseHeaders(resp string) (int, int, error) {
	fmt.Println("prh", resp)
	all := strings.Split(resp, "=")[1]
	rr := strings.Split(all, "-")
	if len(rr) < 2 {
		return -1, -1, errors.New("Bad format at `Content-Range` header")
	} 
	rra := strings.Split(rr[1], "/")
	if len(rra) < 2 {
		return -1, -1, errors.New("Bad format at `Content-Range` header")
	}
	_f, e := strconv.Atoi(rr[0])
	if e != nil {
		return -1, -1, errors.New("Bad format at `Content-Range` header (range from)")
	}
	_t, e:= strconv.Atoi(rra[0])
	if e != nil {
		return -1, -1, errors.New("Bad format at `Content-Range` header (range to)")
	}
	_a, e:=strconv.Atoi(rra[1])
	if e != nil {
		return -1, -1, errors.New("Bad format at `Content-Range` header (range all)")
	}
	return _t - _f, _a, nil
}

func (ct *RangedRetrieableTransport) roundTripErrorStep(req *http.Request, inputBodyBytes []byte) (*http.Response, error, []byte)  {
	resp := &http.Response{}

	allCount, receivedCount := 0, -1
	if len(inputBodyBytes) > 0 {
		receivedCount = len(inputBodyBytes)
		allCount = receivedCount * 2
	}
	fmt.Println("rt e step", receivedCount, allCount)
    for ; receivedCount < allCount ; {
		if receivedCount > 0  {
			r := fmt.Sprintf("bytes=%v-%v", receivedCount, receivedCount + (allCount / 10) )
			fmt.Println("rt e step r", r)
			req.Header.Set("Range", r)
		} else {
			req.Header.Set("Range", "bytes=0")
		}

        resp, err := ct.RoundTripper.RoundTrip(req)
		if err != nil {
			fmt.Println("rt e step err", receivedCount, allCount)
			return nil, err, inputBodyBytes
        } 

		if contentRange := resp.Header.Get("Content-Range"); contentRange != "" {
			currentReceivedCount, _allCount, err := processResponseHeaders(contentRange)
			if err != nil {
				panic(err)
			}
			allCount = _allCount
			receivedCount = receivedCount + currentReceivedCount
			_bodyBytes, err := io.ReadAll(resp.Body)
			inputBodyBytes = append(inputBodyBytes, _bodyBytes...)
			fmt.Println("rt e step acc", len(inputBodyBytes), resp, err, allCount, receivedCount)
			continue
		}
		fmt.Println("rt e step res", resp.Header)
		return resp, nil, nil
    }
	resp.Body = io.NopCloser(bytes.NewReader(inputBodyBytes))
	return resp, nil, nil

}
func (ct *RangedRetrieableTransport) commRoundTrip(req *http.Request, comm chan bool) (resp *http.Response, err error) {
	loadedBytes := []byte{}
	for ;; {
		fmt.Println("comm rt >", len(loadedBytes), resp, err)
		select {
			case <-comm:return nil, nil
			default:
				resp, err, loadedBytes = ct.roundTripErrorStep(req, loadedBytes)
				fmt.Println("comm rt <", len(loadedBytes), resp, err)
				if err != nil {
					continue
				}
				if loadedBytes != nil {
					continue
				}
				return 
			}
	}
}
type rtResp struct {
	Resp *http.Response
	Err error
}
func (ct *RangedRetrieableTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	resultChan := make(chan rtResp) 
	commChan := make(chan bool)

	go func() {
		resp, err := ct.commRoundTrip(req, commChan)
		resultChan <- rtResp{Resp:resp, Err:err}
	}()

	select {
	case <- ct.ctx.Done():
		commChan <- true
	case resp := <- resultChan:
		return resp.Resp, resp.Err
	}
	return 
}

var timeToWait = 10 * time.Second

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), timeToWait)
	defer func() {
		cancel()
	}()
	client := &http.Client{
		Transport: NewRangedRetrieableTransport(ctx, http.DefaultTransport.(*http.Transport), ),
	}
	resp, err := client.Get("http://localhost:6000/data")
	if err != nil {
		fmt.Printf("Error at response: %v\n", err)
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error at read response: %v\n", err)
		return
	}
	fmt.Printf("Loaded %v bytes\n", len(body))
}