package main

import (
	"flag"
	"fmt"
	"os"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/business/usecase"
	"github.com/forest33/tapir/pkg/structs"
)

const (
	commandInit   = "init"
	commandCreate = "create"
	commandExport = "export"
	commandHelp   = "help"
)

type commandData struct {
	serverHost       string
	serverKey        string
	encryption       string
	name             string
	password         string
	compression      string
	compressionLevel int
}

func parseCommandLine() {
	var (
		err     error
		fs      *flag.FlagSet
		data    = &commandData{}
		command = os.Args[1]
	)

	commandHandlers := map[string]func(*commandData){
		commandInit:   handlerInit,
		commandCreate: handlerCreate,
		commandExport: handlerExport,
	}

	switch command {
	case commandInit:
		fs = flag.NewFlagSet(commandInit, flag.ExitOnError)
		fs.StringVar(&data.serverHost, "host", "", "server hostname or IP address")
		fs.StringVar(&data.serverKey, "key", "", "server encryption key")
		fs.StringVar(&data.encryption, "encryption", "aes-256-ecb", "encryption algorithm (none, aes-256-ecb, aes-256-gcm)")
	case commandCreate:
		fs = flag.NewFlagSet(commandCreate, flag.ExitOnError)
		fs.StringVar(&data.name, "name", "", "user name")
		fs.StringVar(&data.password, "password", "", "user password")
		fs.StringVar(&data.compression, "compression", "none", "compression algorithm (none, lz4, lzo, zstd)")
		fs.IntVar(&data.compressionLevel, "compression-level", 0, "compression level (only if using zstd compression)")
	case commandExport:
		fs = flag.NewFlagSet(commandExport, flag.ExitOnError)
		fs.StringVar(&data.name, "name", "", "user name")
	case commandHelp:
		printHelp()
		os.Exit(0)
	default:
		fmt.Printf("Unknown command %s\n", os.Args[1])
		printHelp()
		os.Exit(1)
	}
	if err = fs.Parse(os.Args[2:]); err != nil {
		zlog.Fatal(err)
	}

	commandHandlers[command](data)
}

func handlerInit(data *commandData) {
	if data.serverHost == "" {
		zlog.Fatalf("empty server host name or IP address")
	}
	if _, ok := usecase.Encryptors[entity.EncryptorMethod(data.encryption)]; !ok {
		zlog.Fatalf("unknown encryption method %s", data.encryption)
	}

	cfg.ServerHost = data.serverHost
	cfg.Tunnel.Encryption = entity.EncryptorMethod(data.encryption)

	if data.serverKey != "" {
		cfg.Authentication.Key = data.serverKey
	} else if cfg.Authentication.Key == "" && cfg.Tunnel.Encryption.KeySize() > 0 {
		cfg.Authentication.Key = entity.GetRandomString(cfg.Tunnel.Encryption.KeySize())
	}

	cfg.System.Shell = structs.If(cfg.System.Shell == "", entity.GetSystemShell(), cfg.System.Shell)

	cfgHandler.Update(cfg)
	cfgHandler.Save()

	zlog.Info().Msg("initialization successfully complete")
}

func handlerCreate(data *commandData) {
	if data.name == "" {
		zlog.Fatalf("empty user name")
	}
	if data.password == "" {
		zlog.Fatalf("empty user password")
	}
	if slices.IndexFunc(cfg.Users, func(u *entity.User) bool { return u.Name == data.name }) != -1 {
		zlog.Fatalf("user already exists")
	}

	cfg.Users = append(cfg.Users, &entity.User{
		Name:     data.name,
		Password: data.password,
	})

	cfgHandler.Update(cfg)
	cfgHandler.Save()

	zlog.Info().Msg("user successfully created")
}

func handlerExport(data *commandData) {
	var user *entity.User
	if idx := slices.IndexFunc(cfg.Users, func(u *entity.User) bool { return u.Name == data.name }); idx == -1 {
		zlog.Fatalf("user not exists")
	} else {
		user = cfg.Users[idx]
	}

	conn := &entity.ClientConnection{
		Name:           fmt.Sprintf("%s [%s]", cfg.ServerHost, user.Name),
		Server:         cfg.Network.Clone(),
		Authentication: cfg.Authentication.Clone(),
		User: &entity.User{
			Name:     user.Name,
			Password: user.Password,
		},
		Tunnel: cfg.Tunnel.Clone(),
	}

	conn.Server.Host = cfg.ServerHost
	conn.Tunnel.InterfaceUp = entity.DefaultClientInterfaceUp
	conn.Tunnel.InterfaceDown = entity.DefaultClientInterfaceDown
	conn.Tunnel.AddrMin = ""
	conn.Tunnel.AddrMax = ""

	buf, err := yaml.Marshal(conn)
	if err != nil {
		zlog.Fatalf("failed to import connection")
	}

	fmt.Print(string(buf))
}

func printHelp() {
	fmt.Printf("Usage: ./server command args\n")
	fmt.Printf(" init	- initialize server configuration\n")
	fmt.Printf(" create	- create user\n")
	fmt.Printf(" export	- retrieve the client configuration\n")
	fmt.Printf(" help	- show this help\n")
	fmt.Printf("Get help for a specific command: ./server command -h\n")
}
