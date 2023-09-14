package main

import (
	"context"
	"gopkg.in/yaml.v3"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Dreamacro/clash/adapter"
	"github.com/Dreamacro/clash/constant"
	chttp "github.com/Dreamacro/clash/listener/http"
	"golang.org/x/net/http/httpproxy"
)

const (
	ConfPath     = "./config.yaml"
	WeChatDomain = "https://qyapi.weixin.qq.com"
)

type Config struct {
	ListenPort string         `yaml:"listenPort"`
	ClashPort  string         `yaml:"clashPort"`
	Proxy      map[string]any `yaml:"proxy"`
}

func main() {
	f, err := os.Open(ConfPath)
	if err != nil {
		panic(err)
	}
	cfg := new(Config)
	if err = yaml.NewDecoder(f).Decode(cfg); err != nil {
		panic(err)
	}
	go clash(cfg.ClashPort, cfg.Proxy)
	ct := newClashTransport(cfg.ClashPort)
	go remoteIPCheck(ct)
	reverseProxy(WeChatDomain, cfg.ListenPort, ct)
}

func reverseProxy(domain, port string, ct http.RoundTripper) {
	u, _ := url.Parse(domain)
	s := httputil.NewSingleHostReverseProxy(u)
	rp := &httputil.ReverseProxy{
		Director: func(request *http.Request) {
			s.Director(request)
			request.Host = u.Host
		},
		Transport: ct,
	}

	http.HandleFunc("/ip", func(writer http.ResponseWriter, request *http.Request) {
		var l []string
		for ip := range ips {
			l = append(l, ip)
		}
		sort.Slice(l, func(i, j int) bool {
			return l[i] < l[j]
		})
		writer.Write([]byte(strings.Join(l, ";")))
	})

	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		rp.ServeHTTP(writer, request)
	})

	l := net.JoinHostPort("", port)
	log.Println("[proxy]: listen at:", l)
	log.Fatal(http.ListenAndServe(l, nil))
}

func newClashTransport(port string) *http.Transport {
	d := &net.Dialer{
		Timeout: time.Second,
	}
	c := &httpproxy.Config{
		HTTPProxy:  "http://127.0.0.1:" + port,
		HTTPSProxy: "http://127.0.0.1:" + port,
	}
	pf := c.ProxyFunc()
	p := func(req *http.Request) (*url.URL, error) {
		return pf(req.URL)
	}
	return &http.Transport{
		Proxy:       p,
		DialContext: d.DialContext,
	}
}

func clash(port string, v map[string]any) {
	p, err := adapter.ParseProxy(v)
	if err != nil {
		panic(err)
	}

	in := make(chan constant.ConnContext, 100)
	defer close(in)
	l, err := chttp.New(net.JoinHostPort("127.0.0.1", port), in)
	if err != nil {
		panic(err)
	}
	defer l.Close()

	log.Println("[clash]: listen at:", l.Address())
	log.Println("[clash]: will forward to:", p.Addr())

	for c := range in {
		conn := c
		metadata := conn.Metadata()
		log.Println("[clash]: request to:", metadata.RemoteAddress())
		go func() {
			remote, err := p.DialContext(context.Background(), metadata)
			if err != nil {
				log.Println("[clash]: dial error:", err.Error())
				return
			}
			relay(remote, conn.Conn())
		}()
	}
}

func relay(l, r net.Conn) {
	go io.Copy(l, r)
	io.Copy(r, l)
}

var ips = map[string]bool{}

func remoteIPCheck(ct http.RoundTripper) {
	cli := &http.Client{Transport: ct}

	var f = func() {
		resp, err := cli.Get("http://api4.ipify.org")
		if err != nil {
			log.Println("[ip]: check ip err", err)
			return
		}
		defer resp.Body.Close()
		d, _ := io.ReadAll(resp.Body)
		ip := string(d)
		if !ips[ip] {
			ips[ip] = true
			log.Println("[ip]: get new remote ip:", ip)
		}
	}
	time.Sleep(time.Second)
	f()
	for range time.Tick(time.Minute) {
		f()
	}
}
