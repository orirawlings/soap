package soap

import (
	"bytes"
	"encoding/xml"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_ServeHTTP(t *testing.T) {
	soapSrv := NewServer()
	soapSrv.UseSoap11() // just for testing
	soapSrv.RegisterHandler(
		"/pathTo",
		"testPostAction",
		"fooRequest",
		func() interface{} {
			return &FooRequest{}
		},
		func(request interface{}, w http.ResponseWriter, httpRequest *http.Request) (interface{}, error) {
			fooRequest := request.(*FooRequest)
			return &FooResponse{
				Bar: "Hello \"" + fooRequest.Foo + "\"",
			}, nil
		},
	)
	srv := httptest.NewServer(soapSrv)
	defer srv.Close()

	// the NewClient is incompatible to the server. why? because of the SOAP
	// namespace and its check. due to backwards compatibility reasons the
	// structs in this package can't be changed.

	postFn := func(t *testing.T, postBody []byte) *http.Response {
		body := ioutil.NopCloser(bytes.NewReader(postBody))

		req, err := http.NewRequest("POST", srv.URL+"/pathTo", body)
		require.NoError(t, err)
		req.Header.Add("Content-Type", SoapContentType11)
		req.Header.Add("SOAPAction", "testPostAction")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		return resp
	}

	t.Run("request succeeds", func(t *testing.T) {
		resp := postFn(t, []byte(`<SOAP:Envelope xmlns:SOAP="http://schemas.xmlsoap.org/soap/envelope/">
    <Header xmlns="http://schemas.xmlsoap.org/soap/envelope/"></Header>
    <Body xmlns="http://schemas.xmlsoap.org/soap/envelope/">
        <fooRequest>
            <Foo>i am foo</Foo>
        </fooRequest>
    </Body>
</SOAP:Envelope>`))
		responseEnvelope := &Envelope{
			Body: Body{
				Content: &FooResponse{},
			},
		}

		require.NoError(t, xml.NewDecoder(resp.Body).Decode(responseEnvelope))
		assert.Exactly(t, "Hello \"i am foo\"", responseEnvelope.Body.Content.(*FooResponse).Bar)
	})

	t.Run("request failed", func(t *testing.T) {
		resp := postFn(t, []byte(`<SOAP:Envelope xmlns:SOAP="http://schemas.xmlsoap.org/soap/envelope/">
    <Header xmlns="http://schemas.xmlsoap.org/soap/envelope/"></Header>
    <Body xmlns="http://schemas.xmlsoap.org/soap/envelope/">
        <barRequest>
            <Foo>i am foo</Foo>
        </barRequest>
    </Body>
</SOAP:Envelope>`))

		responseEnvelope := &Envelope{
			Body: Body{Content: &dummyContent{}},
		}

		require.NoError(t, xml.NewDecoder(resp.Body).Decode(responseEnvelope))
		assert.Exactly(t, "no action handler for content type: \"barRequest\"", responseEnvelope.Body.Fault.String)
	})
}

func ExampleServer() {
	type FooRequest struct {
		XMLName xml.Name `xml:"FooRequest"`
		Foo     string
	}

	// BarResponse a simple response
	type BarResponse struct {
		Bar string
	}

	soapServer := NewServer()
	soapServer.Log = log.Println
	soapServer.RegisterHandler(
		"/pathTo",
		"operationFoo", // SOAPAction
		"FooRequest",   // tagname of soap body content
		// RequestFactoryFunc - give the server sth. to unmarshal the request into
		func() interface{} {
			return &FooRequest{}
		},
		// OperationHandlerFunc - do something
		func(request interface{}, w http.ResponseWriter, httpRequest *http.Request) (response interface{}, err error) {
			FooRequest := request.(*FooRequest)
			BarResponse := &BarResponse{
				Bar: "Hello \"" + FooRequest.Foo + "\"",
			}
			response = BarResponse
			return
		},
	)
	_ = http.ListenAndServe(":8080", soapServer)
}
