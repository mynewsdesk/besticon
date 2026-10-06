package besticon

import (
	"errors"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"syscall"
	"time"
)

var _ http.RoundTripper = (*httpTransport)(nil)

type httpTransport struct {
	transport http.RoundTripper

	userAgent string
}

func (h *httpTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", h.userAgent)
	return h.transport.RoundTrip(req)
}

func NewDefaultHTTPTransport(userAgent string) http.RoundTripper {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   refuseNonPublicAddress,
	}).DialContext

	return &httpTransport{
		transport: transport,
		userAgent: userAgent,
	}
}

var errNonPublicAddress = errors.New("refusing to connect to non-public address")

// Runs after DNS resolution, so redirects and rebinding are covered too
func refuseNonPublicAddress(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if ip := net.ParseIP(host); ip == nil || !isPublicIP(ip) {
		return errNonPublicAddress
	}
	return nil
}

var nonPublicNetworks = mustParseCIDRs(
	"0.0.0.0/8",
	"100.64.0.0/10",
	"192.0.0.0/24",
	"198.18.0.0/15",
	"240.0.0.0/4",
	"64:ff9b::/96",
	"64:ff9b:1::/48",
)

func isPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() {
		return false
	}
	for _, network := range nonPublicNetworks {
		if network.Contains(ip) {
			return false
		}
	}
	return true
}

func mustParseCIDRs(cidrs ...string) []*net.IPNet {
	networks := make([]*net.IPNet, len(cidrs))
	for i, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(err)
		}
		networks[i] = network
	}
	return networks
}

func NewDefaultHTTPClient() *http.Client {
	return &http.Client{
		Timeout:   5 * time.Second,
		Jar:       mustInitCookieJar(),
		Transport: NewDefaultHTTPTransport("Mozilla/5.0 (iPhone; CPU iPhone OS 10_0 like Mac OS X) AppleWebKit/602.1.38 (KHTML, like Gecko) Version/10.0 Mobile/14A5297c Safari/602.1"),
	}
}

func (b *Besticon) Get(urlstring string) (*http.Response, error) {
	u, e := url.Parse(urlstring)
	if e != nil {
		return nil, e
	}
	// Maybe we can get rid of this conversion someday
	// https://github.com/golang/go/issues/13835
	u.Host, e = idna.ToASCII(u.Host)
	if e != nil {
		return nil, e
	}

	req, e := http.NewRequest("GET", u.String(), nil)
	if e != nil {
		return nil, e
	}

	start := time.Now()
	resp, err := b.httpClient.Do(req)
	end := time.Now()
	duration := end.Sub(start)

	b.logger.LogResponse(req, resp, duration, err)

	return resp, err
}

func (b *Besticon) GetBodyBytes(r *http.Response) ([]byte, error) {
	limitReader := io.LimitReader(r.Body, b.maxResponseBodySize)
	data, e := ioutil.ReadAll(limitReader)
	r.Body.Close()

	if int64(len(data)) >= b.maxResponseBodySize {
		return nil, errors.New("body too large")
	}
	return data, e
}

func mustInitCookieJar() *cookiejar.Jar {
	options := cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	}
	jar, e := cookiejar.New(&options)
	if e != nil {
		panic(e)
	}

	return jar
}
