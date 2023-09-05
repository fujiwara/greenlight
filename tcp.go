package greenlight

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"regexp"
	"time"
)

var (
	DefaultTCPMaxBytes = 32 * 1024
)

type TCPCheckConfig struct {
	Host               string `yaml:"host"`
	Port               string `yaml:"port"`
	Send               string `yaml:"send"`
	Quit               string `yaml:"quiet"`
	MaxBytes           int    `yaml:"max_bytes"`
	ExpectPattern      string `yaml:"expect_pattern"`
	TLS                bool   `yaml:"tls"`
	NoCheckCertificate bool   `yaml:"no_check_certificate"`
}

type TCPChecker struct {
	Host               string
	Port               string
	Send               string
	Quit               string
	MaxBytes           int
	ExpectPattern      *regexp.Regexp
	Timeout            time.Duration
	TLS                bool
	NoCheckCertificate bool

	name string
}

func (p *TCPChecker) Name() string {
	return p.name
}

func NewTCPChecker(cfg *CheckConfig) (*TCPChecker, error) {
	p := &TCPChecker{
		Timeout:            cfg.Timeout,
		MaxBytes:           cfg.TCP.MaxBytes,
		TLS:                cfg.TCP.TLS,
		NoCheckCertificate: cfg.TCP.NoCheckCertificate,
		Host:               cfg.TCP.Host,
		Port:               cfg.TCP.Port,
		Send:               cfg.TCP.Send,
	}
	if cfg.TCP.ExpectPattern != "" {
		pt, err := regexp.Compile(cfg.TCP.ExpectPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid expect_pattern: %w", err)
		}
		p.ExpectPattern = pt
	}
	if p.MaxBytes == 0 {
		p.MaxBytes = DefaultTCPMaxBytes
	}
	return p, nil
}

func (p *TCPChecker) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()

	addr := net.JoinHostPort(p.Host, p.Port)
	conn, err := dialTCP(ctx, addr, p.TLS, p.NoCheckCertificate, p.Timeout)
	if err != nil {
		return fmt.Errorf("tcp connect failed: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(p.Timeout))

	log.Println("[debug] connected", addr)
	if p.Send != "" {
		log.Println("[debug] send", p.Send)
		_, err := io.WriteString(conn, p.Send)
		if err != nil {
			return fmt.Errorf("tcp send failed: %w", err)
		}
	}
	if p.ExpectPattern != nil {
		buf := make([]byte, p.MaxBytes)
		r := bufio.NewReader(conn)
		n, err := r.Read(buf)
		if err != nil {
			return fmt.Errorf("tcp read failed: %w", err)
		}
		log.Println("[debug] read", string(buf[:n]))

		if !p.ExpectPattern.Match(buf[:n]) {
			return fmt.Errorf("tcp unexpected response: %s", string(buf[:n]))
		}
	}
	if p.Quit != "" {
		log.Println("[debug]", p.Quit)
		io.WriteString(conn, p.Quit)
	}
	return nil
}

func dialTCP(ctx context.Context, address string, useTLS bool, noCheckCertificate bool, timeout time.Duration) (net.Conn, error) {
	d := &net.Dialer{Timeout: timeout}
	if useTLS {
		td := &tls.Dialer{
			NetDialer: d,
			Config: &tls.Config{
				InsecureSkipVerify: noCheckCertificate,
			},
		}
		return td.DialContext(ctx, "tcp", address)
	}
	return d.DialContext(ctx, "tcp", address)
}
