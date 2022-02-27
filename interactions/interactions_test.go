package reminteractions

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInteractions(t *testing.T) {

	//test incorrect signature
	params := ""
	writer := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/interactions", strings.NewReader(params))
	request.Header.Set("x-signature-ed25519", "fb9b94d1ea3b26ac9cdac86c5a20e152ed999eec22d9b1a040506d15f4cbd9ac10fc5916dc045764914779f7e000ac7a177fab9b009e3ffe1a507")
	request.Header.Set("x-signature-timestamp", "16990473")

	interactions(writer, request)

	if writer.Code != http.StatusUnauthorized {
		t.Errorf("Expected %d, got %d:%s\n", http.StatusUnauthorized, writer.Code, writer.Body)
	}

	//test incorrect signature
	params = ""
	writer = httptest.NewRecorder()
	request = httptest.NewRequest("POST", "/interactions", strings.NewReader(params))
	request.Header.Set("x-signature-ed25519", "fb9b94d1ea3b26ac9cdac86c5a20e152ed999ee1a4e2c75b2cc22d9b1a040506d15f4cbd9ac10fc5916dc045764914779f7e000ac7a177fab9b009e3ffe1a507")
	request.Header.Set("x-signature-timestamp", "1645990473")

	interactions(writer, request)

	if writer.Code != http.StatusUnauthorized {
		t.Errorf("Expected %d, got %d:%s\n", http.StatusUnauthorized, writer.Code, writer.Body)
	}

	//test correct signature
	params = `{"application_id":"541298511430287395","id":"947577467147788288","token":"aW50ZXJhY3Rpb246OTQ3NTc3NDY3MTQ3Nzg4Mjg4OnNMVURXemg3VGwwNVlIOHBHYTh1SjNydlhvVnJzYXJKNVNlZFBHQXFxQjRaV0w3TGIzMnlZeERNMThIdTI2Z2l6Y2lPcnZTOXNDNWJxRHJ4ZmZMdUpOUWk5UUZzdktDNnFJdnlJY0dTVXYyM05EeFc1VWN5b0dEdEtaeFNXM1hD","type":1,"user":{"avatar":"a_91417d8d7fa6a87bdcbb85c4551b40c4","discriminator":"2404","id":"196249128286552064","public_flags":0,"username":"Themex"},"version":1}`
	writer = httptest.NewRecorder()
	request = httptest.NewRequest("POST", "/interactions", strings.NewReader(params))
	request.Header.Set("x-signature-ed25519", "78ceb6c6e809994af8b44fd63f838e3432e28dd292097eeed094d55272452b4a65634fd3e3e9b8bc7ba4f6f42b2ddefb13b62cb41c0a372a96e20ec25424450f")
	request.Header.Set("x-signature-timestamp", "1645990473")

	interactions(writer, request)

	if writer.Code != http.StatusOK {
		t.Errorf("Expected %d, got %d:%s\n", http.StatusOK, writer.Code, writer.Body)
	}

	rawBody, err := io.ReadAll(writer.Body)
	if err != nil {
		t.Errorf("Failed to read body: %s\n", err)
	}
	body := string(rawBody)

	if body != `{"type":1}` {
		t.Errorf(`Expected '{"type":1}', got %s\n`, body)
	}

}
