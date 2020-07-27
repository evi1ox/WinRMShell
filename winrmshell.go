package main

import (
	"bufio"
	"encoding/base64"
	_ "errors"
	"flag"
	"fmt"
	"os"
	_ "reflect"
	"strings"
	"time"

	"github.com/masterzen/winrm"
	"github.com/mattn/go-isatty"
)

func main() {
	var (
		hostname string
		user     string
		pass     string
		ntlm     bool
		shell    bool
		cmd      string
		port     int
		encoded  bool
		https    bool
		insecure bool
		timeout  string
	)

	flag.StringVar(&hostname, "i", "127.0.0.1", "WinRM Host")
	flag.StringVar(&user, "u", "administrator", "WinRM Username")
	flag.StringVar(&pass, "p", "password", "WinRM Password")
	flag.BoolVar(&ntlm, "ntlm", false, "Use NTLM")
	flag.BoolVar(&encoded, "encoded", false, "Use Base64 Encoded Password")
	flag.BoolVar(&shell,"shell",false,"Use Interactive Shell Terminal")
	flag.IntVar(&port, "port", 5985, "WinRM Port")
	flag.BoolVar(&https, "https", false, "Use Https")
	flag.BoolVar(&insecure, "insecure", true, "Skip SSL Validation")
	flag.StringVar(&timeout, "timeout", "0s", "Connection Timeout")
	flag.StringVar(&cmd, "cmd", "whoami", "Run Command Exec")

	flag.Parse()

	if encoded {
		data, err := base64.StdEncoding.DecodeString(pass)
		check(err)
		pass = strings.TrimRight(string(data), "\r\n")
	}

	var (
		err            error
		connectTimeout time.Duration
	)

	connectTimeout, err = time.ParseDuration(timeout)
	check(err)

	endpoint := winrm.NewEndpoint(hostname, port, https, insecure, nil, nil, nil, connectTimeout)

	params := winrm.DefaultParameters

	if ntlm {
		params.TransportDecorator = func() winrm.Transporter { return &winrm.ClientNTLM{} }
	}

	client, err := winrm.NewClientWithParameters(endpoint, user, pass, params)
	check(err)

	exitCode := 0
	if shell {
		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Print("$ ")
			cmdString, err := reader.ReadString('\n')
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}

			cmdString = strings.TrimSuffix(cmdString, "\n")
			if cmdString == "exit"{
				os.Exit(exitCode)
			}

			if isatty.IsTerminal(os.Stdin.Fd()) {
				exitCode, err = client.Run(cmdString, os.Stdout, os.Stderr)
			} else {
				exitCode, err = client.RunWithInput(cmdString, os.Stdout, os.Stderr, os.Stdin)
			}
			check(err)

			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}else {
		if isatty.IsTerminal(os.Stdin.Fd()) {
			exitCode, err = client.Run(cmd, os.Stdout, os.Stderr)
		} else {
			exitCode, err = client.RunWithInput(cmd, os.Stdout, os.Stderr, os.Stdin)
		}
		check(err)
	}


	os.Exit(exitCode)
}

func check(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

//func IsNil(i interface{}) bool {
//
//	vi := reflect.ValueOf(i)
//
//	if vi.Kind() == reflect.Ptr {
//
//		return vi.IsNil()
//
//	}
//
//	return false
//
//}
