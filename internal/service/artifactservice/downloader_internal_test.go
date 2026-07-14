package artifactservice

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

type fakeResolver struct {
	mu      sync.Mutex
	answers map[string][][]net.IPAddr
}

func (r *fakeResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IPAddr{{IP: ip}}, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	seq := r.answers[host]
	if len(seq) == 0 {
		return nil, fmt.Errorf("unexpected host lookup %q", host)
	}
	answer := seq[0]
	if len(seq) > 1 {
		r.answers[host] = seq[1:]
	}
	return answer, nil
}

type pipeDialer struct {
	mu        sync.Mutex
	responses []string
	addresses []string
}

func (d *pipeDialer) DialContext(ctx context.Context, _ string, address string) (net.Conn, error) {
	d.mu.Lock()
	d.addresses = append(d.addresses, address)
	if len(d.responses) == 0 {
		d.mu.Unlock()
		return nil, errors.New("unexpected dial")
	}
	response := d.responses[0]
	d.responses = d.responses[1:]
	d.mu.Unlock()

	client, server := net.Pipe()
	go func() {
		defer server.Close()
		req, err := http.ReadRequest(bufio.NewReader(server))
		if err == nil && req.Body != nil {
			_ = req.Body.Close()
		}
		_, _ = server.Write([]byte(response))
	}()
	go func() {
		<-ctx.Done()
		_ = client.Close()
	}()
	return client, nil
}

func (d *pipeDialer) Addresses() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.addresses...)
}

func response(status int, headers map[string]string, body string) string {
	var b strings.Builder
	b.WriteString("HTTP/1.1 ")
	b.WriteString(strconv.Itoa(status))
	b.WriteString(" ")
	b.WriteString(http.StatusText(status))
	b.WriteString("\r\n")
	for k, v := range headers {
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(v)
		b.WriteString("\r\n")
	}
	if _, ok := headers["Content-Length"]; !ok {
		b.WriteString("Content-Length: ")
		b.WriteString(strconv.Itoa(len(body)))
		b.WriteString("\r\n")
	}
	b.WriteString("\r\n")
	b.WriteString(body)
	return b.String()
}

func publicAddr(ip string) []net.IPAddr {
	return []net.IPAddr{{IP: net.ParseIP(ip)}}
}

func TestHTTPDownloaderBlocksPrivateAddressRanges(t *testing.T) {
	d := newHTTPDownloader()
	for _, rawURL := range []string{
		"http://10.0.0.1/artifact",
		"http://127.0.0.1/artifact",
		"http://localhost/artifact",
		"http://169.254.169.254/latest/meta-data/",
		"http://100.64.0.1/artifact",
		"http://0.0.0.0/artifact",
	} {
		t.Run(rawURL, func(t *testing.T) {
			_, _, err := d.Download(context.Background(), rawURL)
			if err == nil {
				t.Fatal("expected private address to be blocked")
			}
		})
	}
}

func TestHTTPDownloaderBlocksPrivateRedirect(t *testing.T) {
	d := newHTTPDownloader()
	d.resolver = &fakeResolver{answers: map[string][][]net.IPAddr{
		"provider.example": {publicAddr("203.0.113.10"), publicAddr("203.0.113.10")},
	}}
	dialer := &pipeDialer{responses: []string{
		response(http.StatusFound, map[string]string{"Location": "http://127.0.0.1/private"}, ""),
	}}
	d.dialContext = dialer.DialContext

	_, _, err := d.Download(context.Background(), "http://provider.example/artifact")
	if err == nil {
		t.Fatal("expected private redirect target to be blocked")
	}
	if got := dialer.Addresses(); len(got) != 1 {
		t.Fatalf("private redirect must be blocked before second dial, dials=%v", got)
	}
}

func TestHTTPDownloaderDialsResolvedPublicIP(t *testing.T) {
	d := newHTTPDownloader()
	d.resolver = &fakeResolver{answers: map[string][][]net.IPAddr{
		"provider.example": {publicAddr("203.0.113.10"), publicAddr("203.0.113.10")},
	}}
	dialer := &pipeDialer{responses: []string{
		response(http.StatusOK, map[string]string{"Content-Type": "video/mp4"}, "ok"),
	}}
	d.dialContext = dialer.DialContext

	data, contentType, err := d.Download(context.Background(), "http://provider.example/artifact")
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if string(data) != "ok" || contentType != "video/mp4" {
		t.Fatalf("download = %q %q, want ok video/mp4", string(data), contentType)
	}
	if got := dialer.Addresses(); len(got) != 1 || got[0] != "203.0.113.10:80" {
		t.Fatalf("dial addresses = %v, want vetted IP only", got)
	}
}

func TestHTTPDownloaderDisablesProxyResolutionByDefault(t *testing.T) {
	d := newHTTPDownloader()
	transport, ok := d.client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport = %T, want *http.Transport", d.client.Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("default downloader must dial vetted target IPs directly, not via environment proxy")
	}
}

func TestHTTPDownloaderDownloadsPublicProviderThroughHTTPServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatalf("server addr: %v", err)
	}

	d := newHTTPDownloader()
	d.resolver = &fakeResolver{answers: map[string][][]net.IPAddr{
		"provider.example": {publicAddr("203.0.113.10"), publicAddr("203.0.113.10")},
	}}
	var (
		mu      sync.Mutex
		dialed  []string
		dialer  net.Dialer
		rawURL  = "http://provider.example:" + port + "/artifact"
		testURL = server.Listener.Addr().String()
	)
	d.dialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		mu.Lock()
		dialed = append(dialed, address)
		mu.Unlock()
		return dialer.DialContext(ctx, network, testURL)
	}

	data, contentType, err := d.Download(context.Background(), rawURL)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if string(data) != "ok" || contentType != "image/png" {
		t.Fatalf("download = %q %q, want ok image/png", string(data), contentType)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(dialed) != 1 || dialed[0] != "203.0.113.10:"+port {
		t.Fatalf("dial addresses = %v, want vetted public IP", dialed)
	}
}

func TestHTTPDownloaderBlocksDNSRebindingBeforeDial(t *testing.T) {
	d := newHTTPDownloader()
	d.resolver = &fakeResolver{answers: map[string][][]net.IPAddr{
		"provider.example": {publicAddr("203.0.113.10"), publicAddr("127.0.0.1")},
	}}
	dialer := &pipeDialer{responses: []string{
		response(http.StatusOK, nil, "must-not-dial"),
	}}
	d.dialContext = dialer.DialContext

	_, _, err := d.Download(context.Background(), "http://provider.example/artifact")
	if err == nil {
		t.Fatal("expected DNS rebinding to private IP to be blocked")
	}
	if got := dialer.Addresses(); len(got) != 0 {
		t.Fatalf("private rebound IP must be blocked before dial, dials=%v", got)
	}
}

func TestHTTPDownloaderAllowedHostsStillRestrictsEgress(t *testing.T) {
	d := newHTTPDownloader()
	d.setAllowedHosts([]string{"allowed.example"})
	d.resolver = &fakeResolver{answers: map[string][][]net.IPAddr{
		"allowed.example": {publicAddr("203.0.113.10"), publicAddr("203.0.113.10")},
	}}
	dialer := &pipeDialer{responses: []string{
		response(http.StatusOK, nil, "allowed"),
	}}
	d.dialContext = dialer.DialContext

	if _, _, err := d.Download(context.Background(), "http://blocked.example/artifact"); err == nil {
		t.Fatal("expected disallowed host to be blocked")
	}
	data, _, err := d.Download(context.Background(), "http://allowed.example/artifact")
	if err != nil {
		t.Fatalf("allowed host download: %v", err)
	}
	if string(data) != "allowed" {
		t.Fatalf("data = %q, want allowed", string(data))
	}
}

func TestHTTPDownloaderDataURLDoesNotUseNetworkGuards(t *testing.T) {
	d := newHTTPDownloader()
	d.resolver = &fakeResolver{}
	dialer := &pipeDialer{}
	d.dialContext = dialer.DialContext

	data, contentType, err := d.Download(context.Background(), "data:text/plain;base64,aGVsbG8=")
	if err != nil {
		t.Fatalf("download data URL: %v", err)
	}
	if string(data) != "hello" || contentType != "text/plain" {
		t.Fatalf("data url = %q %q, want hello text/plain", string(data), contentType)
	}
	if got := dialer.Addresses(); len(got) != 0 {
		t.Fatalf("data URL must not dial, got %v", got)
	}
}
