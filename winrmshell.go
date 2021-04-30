package main

import (
	"bufio"
	"encoding/base64"
	_ "errors"
	"flag"
	"fmt"
	"github.com/evi1ox/WinRMShell/winrmcp"
	"github.com/malfunkt/iprange"
	"log"
	"net"
	"os"
	_ "reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gookit/color"
	"github.com/masterzen/winrm"
	"github.com/mattn/go-isatty"
	"github.com/spf13/viper"
)

func main() {
	var (
		filelists string
		hostlists string
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
		src_path string
		dst_path string
		err            error
		hostlistExpand []net.IP
		connectTimeout time.Duration
	)
	flag.StringVar(&filelists, "c", "", "Host config file.")
	flag.StringVar(&hostlists, "i", "", "Host to be scanned, supports four formats:\n192.168.1.1\n192.168.1.1-10\n192.168.1.*\n192.168.1.0/24.")
	flag.StringVar(&user, "u", "administrator", "WinRM Username")
	flag.StringVar(&pass, "p", "password", "WinRM Password")
	flag.BoolVar(&ntlm, "ntlm", false, "Use NTLM")
	flag.BoolVar(&encoded, "encoded", false, "Use Base64 Encoded Password")
	flag.BoolVar(&shell,"shell",false,"Use Interactive Shell Terminal")
	flag.IntVar(&port, "P", 5985, "WinRM Port")
	flag.BoolVar(&https, "https", false, "Use Https")
	flag.BoolVar(&insecure, "insecure", true, "Skip SSL Validation")
	flag.StringVar(&timeout, "t", "2", "Connection Timeout")
	flag.StringVar(&cmd, "cmd", "whoami", "Run Command Exec")
	flag.StringVar(&src_path, "src", "", "Source Address")
	flag.StringVar(&dst_path, "dst", "", "Destination address")

	flag.Parse()

	os.Setenv("WINRMCP_DEBUG","yes")

	if filelists != "" {
		configNameList := strings.Split(filelists, ".")
		configName := strings.Join(configNameList[:len(configNameList)-1],".")
		viper.SetConfigName(configName)     //把json文件换成yaml文件，只需要配置文件名 (不带后缀)即可
		viper.AddConfigPath(".")           //添加配置文件所在的路径
		err := viper.ReadInConfig()
		if err != nil {
			fmt.Printf("config file error: %s\n", err)
			os.Exit(1)
		}

		viper.WatchConfig()           //监听配置变化
		viper.OnConfigChange(func(e fsnotify.Event) {
			fmt.Println("[+] 配置发生变更：", e.Name)
		})
		hostlist := viper.GetStringSlice("host")
		user = viper.GetString("host_config.user")
		pass = viper.GetString("host_config.password")
		port = viper.GetInt("host_config.port")
		ntlm = viper.GetBool("host_config.ntlm")
		fmt.Printf("[+] 加载配置文件: %s\n", viper.ConfigFileUsed())
		fmt.Printf("    Host: %v\n", hostlist)
		fmt.Printf("    User: %s\n", user)
		fmt.Printf("    Pass: %s\n", pass)
		fmt.Printf("    Port: %d\n", port)
		fmt.Printf("    Ntlm: %v\n", ntlm)
		for _,ip := range hostlist{
			parseIP := net.ParseIP(ip)
			hostlistExpand = append(hostlistExpand, parseIP)
		}

	}else{
		hostsPattern := `^(([01]?\d?\d|2[0-4]\d|25[0-5])\.){3}([01]?\d?\d|2[0-4]\d|25[0-5])\/(\d{1}|[0-2]{1}\d{1}|3[0-2])$|^(25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[0-9]{1,2})(\.(25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[0-9]{1,2})){3}$`
		hostsRegexp := regexp.MustCompile(hostsPattern)
		checkHost := hostsRegexp.MatchString(hostlists)

		hostsPattern2 := `\b(?:(?:25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9]?[0-9])\.){3}(((2(5[0-5]|[0-4]\d))|[0-1]?\d{1,2})\-((2(5[0-5]|[0-4]\d))|[0-1]?\d{1,2}))\b`
		hostsRegexp2 := regexp.MustCompile(hostsPattern2)
		checkHost2 := hostsRegexp2.MatchString(hostlists)

		hostsPattern3 := `((25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9]?[0-9])\.){3}(\*$)`
		hostsRegexp3 := regexp.MustCompile(hostsPattern3)
		checkHost3 := hostsRegexp3.MatchString(hostlists)

		if hostlists == "" || (checkHost == false && checkHost2 == false && checkHost3 == false){
			flag.Usage()
			return
		}
		hostlist, err := iprange.ParseList(hostlists)
		check(err)
		hostlistExpand = hostlist.Expand()
	}

	if encoded {
		data, err := base64.StdEncoding.DecodeString(pass)
		check(err)
		pass = strings.TrimRight(string(data), "\r\n")
	}

	timeout += "s"
	connectTimeout, err = time.ParseDuration(timeout)
	check(err)


	fmt.Printf("[+] 共执行指令的服务器个数: %d\n",len(hostlistExpand))

	if len(hostlistExpand) != 1 {
		shell = false
	}

	for _,hostname := range hostlistExpand {
		hostname := hostname.String()

		color.Red.Printf("\n[+] %s",hostname)

		if connect := raw_connect(hostname, strconv.Itoa(port),connectTimeout); connect != true {
			fmt.Println("    连接失败")
			continue
		}

		if src_path != "" && dst_path !=""{
			fmt.Println()
			addr := fmt.Sprintf("%s:%d", hostname,port)
			client, err := winrmcp.New(addr, &winrmcp.Config{
				Auth:                  winrmcp.Auth{User: user, Password: pass},
				Https:                 https,
				Insecure:              insecure,
				TLSServerName:         "",
				CACertBytes:           nil,
				OperationTimeout:      connectTimeout,
				MaxOperationsPerShell: 15,
			})

			if err != nil {
				log.Fatal(err)
			}
			client.Copy(src_path, dst_path)

		}else{
			command(hostname,port,user,pass,https,cmd,insecure,connectTimeout,shell,ntlm)
			//exitCode:=command(hostname,port,user,pass,https,cmd,insecure,connectTimeout,shell,ntlm)
			//fmt.Println(exitCode)
		}
	}
}

func command(hostname string ,port int,user string, pass string, https bool,cmd string, insecure bool, connectTimeout time.Duration, shell bool, ntlm bool)(exitCode int){
	endpoint := winrm.NewEndpoint(hostname, port, https, insecure, nil, nil, nil, connectTimeout)

	params := winrm.DefaultParameters

	if ntlm {
		params.TransportDecorator = func() winrm.Transporter { return &winrm.ClientNTLM{} }
	}

	client, err := winrm.NewClientWithParameters(endpoint, user, pass, params)
	check(err)



	if shell {
		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Print("\n$ ")
			cmdString, err := reader.ReadString('\n')
			if err != nil {
				log.Println(os.Stderr, err)
			}

			cmdString = strings.TrimSuffix(cmdString, "\n")
			if cmdString == "exit"{
				return exitCode
			}

			if isatty.IsTerminal(os.Stdin.Fd()) {
				exitCode, err = client.Run(cmdString, os.Stdout, os.Stderr)
			} else {
				exitCode, err = client.RunWithInput(cmdString, os.Stdout, os.Stderr, os.Stdin)
			}

			if err != nil {
				log.Println(os.Stderr, err)
			}
		}
	}else {
		fmt.Printf("\n    ")
		if isatty.IsTerminal(os.Stdin.Fd()) {
			exitCode, err = client.Run(cmd, os.Stdout, os.Stderr)
		} else {
			exitCode, err = client.RunWithInput(cmd, os.Stdout, os.Stderr, os.Stdin)
		}
		if err !=nil{
			log.Println(err)
		}
	}
	return exitCode

	//os.Exit(exitCode)
}

func check(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func raw_connect(host string, port string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil {
		return false
	}
	if conn != nil {
		defer conn.Close()
	}
	return true
}
