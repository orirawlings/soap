package soap

import (
	"bytes"
	"context"
	"encoding/xml"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type FooRequest struct {
	XMLName xml.Name `xml:"fooRequest"`
	Foo     string
}

// FooResponse a simple response
type FooResponse struct {
	Bar string
}

func TestClient_Call(t *testing.T) {
	wantSOAPBody := []byte(`<Envelope xmlns="http://schemas.xmlsoap.org/soap/envelope/">
	<Header xmlns="http://schemas.xmlsoap.org/soap/envelope/"></Header>
	<Body xmlns="http://schemas.xmlsoap.org/soap/envelope/">
		<fooRequest>
			<Foo>hello world</Foo>
		</fooRequest>
	</Body>
</Envelope>`)

	httpSOAPResponse := []byte(`<soap12:Envelope xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
  xmlns:xsd="http://www.w3.org/2001/XMLSchema" 
  xmlns:soap12="http://www.w3.org/2003/05/soap-envelope">
  <soap12:Body>
    <FooResponse xmlns="http://xmlme.com/WebServices">
      <Bar>I love deadlines. I like the whooshing sound they make as they fly by.</Bar>
    </FooResponse>
  </soap12:Body>
</soap12:Envelope>`)

	clientDoFn := func(rt func(r *http.Request) (*http.Response, error)) func(req *http.Request) (*http.Response, error) {
		return (&http.Client{
			Transport: RoundTrip(rt),
		}).Do
	}

	t.Run("without multipart", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			c := NewClient("http://localhorst.ch", &BasicAuth{
				Login:    "test",
				Password: "test",
			})
			c.UserAgent = "ncc-1701-d"
			c.RequestHeaderFn = func(header http.Header) {
				header.Set("X-Answer", "42")
			}
			c.HTTPClientDoFn = clientDoFn(func(r *http.Request) (*http.Response, error) {
				haveBody, _ := ioutil.ReadAll(r.Body)
				assert.Exactly(t, wantSOAPBody, haveBody)
				assert.Exactly(t, "42", r.Header.Get("X-Answer"))
				assert.Exactly(t, "ncc-1701-d", r.Header.Get("User-Agent"))
				return &http.Response{
					StatusCode: 200,
					Body:       ioutil.NopCloser(bytes.NewReader(httpSOAPResponse)),
				}, nil
			})
			req := FooRequest{
				Foo: "hello world",
			}
			var resp FooResponse
			httpResp, err := c.Call(context.Background(), "MySOAPAction", &req, &resp)
			require.NoError(t, err)
			assert.NotNil(t, httpResp)
			assert.Exactly(t, 200, httpResp.StatusCode)
			assert.Exactly(t, FooResponse{Bar: `I love deadlines. I like the whooshing sound they make as they fly by.`}, resp)
		})

		t.Run("no soap body", func(t *testing.T) {
			c := NewClient("http://localhorst.ch", nil)
			c.HTTPClientDoFn = clientDoFn(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Body: ioutil.NopCloser(strings.NewReader(`<?xml version="1.0" encoding="utf-8"?>
<seife12:Envelope xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
  xmlns:xsd="http://www.w3.org/2001/XMLSchema" 
  xmlns:soap12="http://www.w3.org/2003/05/soap-envelope">
  <seife:Body></seife:Body>
</seife:Envelope>`)),
				}, nil
			})
			req := FooRequest{}
			var resp FooResponse
			httpResp, err := c.Call(context.Background(), "MySOAPAction", &req, &resp)
			assert.Nil(t, httpResp)
			assert.EqualError(t, err, "soap/client.go Call(): COULD NOT UNMARSHAL: expected element <Envelope> in name space http://schemas.xmlsoap.org/soap/envelope/ but have seife12\n")
		})
	})
	t.Run("with multipart", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			c := NewClient("http://localhorst.ch", nil)
			c.HTTPClientDoFn = clientDoFn(func(r *http.Request) (*http.Response, error) {
				buf, mw := createMultiPart(t, httpSOAPResponse)
				hdr := http.Header{}
				hdr.Add("Content-Type", mw.FormDataContentType())
				return &http.Response{
					Header:     hdr,
					StatusCode: 200,
					Body:       ioutil.NopCloser(buf),
				}, nil
			})
			req := FooRequest{
				Foo: "hello world",
			}
			var resp FooResponse
			httpResp, err := c.Call(context.Background(), "MySOAPAction", &req, &resp)
			require.NoError(t, err)
			assert.NotNil(t, httpResp)
			assert.Exactly(t, 200, httpResp.StatusCode)
			assert.Exactly(t, FooResponse{Bar: `I love deadlines. I like the whooshing sound they make as they fly by.`}, resp)
		})
		t.Run("no soap found", func(t *testing.T) {
			c := NewClient("http://localhorst.ch", nil)
			c.HTTPClientDoFn = clientDoFn(func(r *http.Request) (*http.Response, error) {
				buf, mw := createMultiPart(t, []byte(`<wrong></wrong>`))
				hdr := http.Header{}
				hdr.Add("Content-Type", mw.FormDataContentType())
				return &http.Response{
					Header:     hdr,
					StatusCode: 200,
					Body:       ioutil.NopCloser(buf),
				}, nil
			})
			req := FooRequest{
				Foo: "hello world",
			}
			var resp FooResponse
			httpResp, err := c.Call(context.Background(), "MySOAPAction", &req, &resp)
			assert.Nil(t, httpResp)
			assert.EqualError(t, err, "multipart message does contain a soapy part")
		})
	})
}

func createMultiPart(t *testing.T, data []byte) (*bytes.Buffer, *multipart.Writer) {
	buf := new(bytes.Buffer)
	w := multipart.NewWriter(buf)

	fw, err := w.CreateFormFile("soap", "test_soap.xml")
	if err != nil {
		t.Fatal(err)
	}

	fw.Write(data)

	// Important if you do not close the multipart writer you will not have a
	// terminating boundry
	w.Close()

	return buf, w
}

func ExampleClient() {
	// FooRequest a simple request
	type FooRequest struct {
		XMLName xml.Name `xml:"fooRequest"`
		Foo     string
	}

	// FooResponse a simple response
	type FooResponse struct {
		Bar string
	}

	client := NewClient("http://127.0.0.1:8080/", nil)
	client.Log = func(msg string, keyString_ValueInterface ...interface{}) {
		keyString_ValueInterface = append(keyString_ValueInterface, msg)
		log.Println(keyString_ValueInterface...)
	} // verbose
	response := &FooResponse{}
	httpResponse, err := client.Call(context.Background(), "operationFoo", &FooRequest{Foo: "hello i am foo"}, response)
	if err != nil {
		panic(err)
	}
	log.Println(response.Bar, httpResponse.Status)
}
