package main

import (
    "fmt"
    "net/http"
	"math/rand"
    "strings"
    "strconv"
    "time"
)
var (
    minRangeSize = 1024
    dataSize = minRangeSize * minRangeSize
)


func generateData() ([]byte){
    token := make([]byte, dataSize)
    rand.Read(token)
    return token
}

var data []byte = generateData()

type Range struct {
    From int
    To int
}

func GetRange(h string) (*Range, error) {
    val := strings.Split(h, "=")[1]
    if strings.Contains(val, "-") {
        fromTo := strings.Split(val, "-")
        from_ , err := strconv.Atoi(fromTo[0])
        if err != nil {
            return nil, err
        }
        to_, err := strconv.Atoi(fromTo[1])
        if err != nil {
            return nil, err
        }
        return &Range{From:from_, To:to_}, nil
    }

    from_, err := strconv.Atoi(val)
    if err != nil {
        return nil, err
    }

    return &Range{From: from_, To: from_ + minRangeSize }, nil 

}

type boolgen struct {
    src       rand.Source
    cache     int64
    remaining int
}

func (b *boolgen) Bool() bool {
    if b.remaining == 0 {
        b.cache, b.remaining = b.src.Int63(), 63
    }

    result := b.cache&0x01 == 1
    b.cache >>= 1
    b.remaining--

    return result
}

func NewRandomBool() *boolgen {
    return &boolgen{src: rand.NewSource(time.Now().UnixNano())}
}

func dataHandler(w http.ResponseWriter, req *http.Request) {
    fmt.Printf("Data will response...\n")

    var bytesRange *Range
    for name, headers := range req.Header {
        if (name == "Range") {
            for _, h := range headers {
                r, err := GetRange(h)
                if err != nil {
                    continue
                }
                bytesRange = r
                fmt.Printf("Have a bytes range: %+v\n", bytesRange)
                break
            } 
        }
    }
    if bytesRange == nil {
        bytesRange = &Range{From: 0, To: minRangeSize}
        fmt.Printf("Haven't a bytes range, make default: %+v\n", bytesRange)
    }

    w.Header().Set("Content-Length", fmt.Sprintf("%v", bytesRange.To - bytesRange.From))
    w.Header().Set("Content-Range", fmt.Sprintf("bytes=%v-%v/%v", bytesRange.From, bytesRange.To, len(data)))
    w.Header().Set("Accept-Range", "bytes") 
    if NewRandomBool().Bool() {
        w.Write(data[bytesRange.From : bytesRange.To - (minRangeSize / 2)])    
        panic("hah") // emulate some error
    } else {
        w.Write(data[bytesRange.From : bytesRange.To])    
    }
    

}

func main() {
    fmt.Println("Will start on :6000 ")
    http.HandleFunc("/", dataHandler)
    http.ListenAndServe(":6000", nil)
}