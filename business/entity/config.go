// Package entity provides entities for business logic.
package entity

import (
	"encoding/json"
	"math/rand"
	"runtime"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/forest33/tapir/pkg/structs"
)

const (
	PortSelectionStrategyNameRandom = "random"
	PortSelectionStrategyNameHash   = "hash"

	CompressionNameNone = "none"
	CompressionNameLZ4  = "lz4"
	CompressionNameLZO  = "lzo"
	CompressionNameZSTD = "zstd"

	DefaultServerConfigFileName = "tapir-server.yaml"
	DefaultClientConfigFileName = "tapir-client.yaml"
)

// ServerConfig server configuration
type ServerConfig struct {
	ServerHost     string                `yaml:"ServerHost" default:""`
	Logger         *LoggerConfig         `yaml:"Logger"`
	Runtime        *RuntimeConfig        `yaml:"Runtime"`
	System         *SystemConfig         `yaml:"System"`
	Network        *NetworkConfig        `yaml:"Network"`
	Tunnel         *TunnelConfig         `yaml:"Tunnel"`
	StreamMerger   *StreamMergerConfig   `yaml:"StreamMerger"`
	Retry          *RetryConfig          `yaml:"Retry"`
	Ack            *AckConfig            `yaml:"Acknowledgement"`
	Authentication *AuthenticationConfig `yaml:"Authentication"`
	Users          []*User               `yaml:"Users"`
	Tracing        *TracingConfig        `yaml:"Tracing,omitempty"`
	Profiler       *ProfilerConfig       `yaml:"Profiler"`
	Rest           *RestConfig           `yaml:"Rest"`
	Statistic      *StatisticConfig      `yaml:"Statistic"`
}

// ClientConfig client configuration
type ClientConfig struct {
	Logger       *LoggerConfig         `yaml:"Logger"`
	Runtime      *RuntimeConfig        `yaml:"Runtime"`
	System       *SystemConfig         `yaml:"System"`
	StreamMerger *StreamMergerConfig   `yaml:"StreamMerger"`
	Retry        *RetryConfig          `yaml:"Retry"`
	Ack          *AckConfig            `yaml:"Acknowledgement"`
	Connections  []*ClientConnection   `yaml:"Connections"`
	Tracing      *TracingConfig        `yaml:"Tracing"`
	Profiler     *ProfilerConfig       `yaml:"Profiler"`
	Application  *ApplicationConfig    `yaml:"Application"`
	GUI          *GUISettings          `yaml:"GUI"`
	IPC          *ClientIPCSettings    `yaml:"IPC"`
	ClientRunner *ClientRunnerSettings `yaml:"ClientRunner"`
	Statistic    *StatisticConfig      `yaml:"Statistic"`
}

// LoggerConfig logger settings
type LoggerConfig struct {
	Level             string `yaml:"level" default:"info"`
	TimeFieldFormat   string `yaml:"timeFieldFormat" default:"2006-01-02T15:04:05.000000"`
	PrettyPrint       *bool  `yaml:"prettyPrint" default:"false"`
	DisableSampling   *bool  `yaml:"disableSampling" default:"true"`
	RedirectStdLogger *bool  `yaml:"redirectStdLogger" default:"true"`
	ErrorStack        *bool  `yaml:"errorStack" default:"true"`
	ShowCaller        *bool  `yaml:"showCaller" default:"false"`
	FileName          string `yaml:"fileName,omitempty" default:""`
}

// RuntimeConfig runtime settings
type RuntimeConfig struct {
	GoMaxProcs int `yaml:"goMaxProcs" default:"0"`
}

// SystemConfig system settings
type SystemConfig struct {
	ClientID string `yaml:"clientId,omitempty" default:""`
	Shell    string `yaml:"shell" default:""`
}

// NetworkConfig server & client network configuration
type NetworkConfig struct {
	Host                  string `yaml:"host,omitempty" default:""`
	PortMin               uint16 `yaml:"portMin" default:"1977"`
	PortMax               uint16 `yaml:"portMax" default:"1986"`
	UseTCP                *bool  `yaml:"useTCP" default:"false"`
	UseUDP                *bool  `yaml:"useUDP" default:"true"`
	ReadBufferSize        int    `yaml:"readBufferSize" default:"131071"`
	WriteBufferSize       int    `yaml:"writeBufferSize" default:"131071"`
	MultipathTCP          *bool  `yaml:"multipathTCP" default:"true"`
	AuthenticationTimeout int    `yaml:"authenticationTimeout" default:"10"`
	HandshakeTimeout      int    `yaml:"handshakeTimeout" default:"10"`
	ResetTimeout          int    `yaml:"resetTimeout" default:"10"`
	MaxConnectionAttempts int    `yaml:"maxConnectionAttempts" default:"3"`
	KeepaliveTimeout      int    `yaml:"keepaliveTimeout" default:"60"`
	KeepaliveInterval     int    `yaml:"keepaliveInterval" default:"2"`
	KeepaliveProbes       int    `yaml:"keepaliveProbes" default:"20"`
	PortSelectionStrategy string `yaml:"portSelectionStrategy" default:"random"`
	Compression           string `yaml:"compression" default:"none"`
	CompressionLevel      int    `yaml:"compressionLevel,omitempty" default:"0"`
	ObfuscateData         *bool  `yaml:"obfuscateData,omitempty" default:"true"`
}

type ClientConnection struct {
	Name           string                `yaml:"Name"`
	Server         *NetworkConfig        `yaml:"Server"`
	Authentication *AuthenticationConfig `yaml:"Authentication"`
	User           *User                 `yaml:"User"`
	Tunnel         *TunnelConfig         `yaml:"Tunnel"`
}

// TunnelConfig VPN startup config
type TunnelConfig struct {
	MTU                    int                 `yaml:"mtu" default:"1400"`
	AddrMin                string              `yaml:"addrMin,omitempty" default:"192.168.30.0"`
	AddrMax                string              `yaml:"addrMax,omitempty" default:"192.168.50.0"`
	InterfaceUp            map[string][]string `yaml:"interfaceUp"`
	InterfaceDown          map[string][]string `yaml:"interfaceDown"`
	NumberOfHandlerThreads int                 `yaml:"numberOfHandlerThreads" default:"4"`
	Encryption             EncryptorMethod     `yaml:"encryption" default:"aes-256-ecb"`
}

// StreamMergerConfig stream merger configuration
type StreamMergerConfig struct {
	ThreadingBy         string  `yaml:"threadingBy" default:"endpoint"`
	WaitingListMaxSize  int     `yaml:"waitingListMaxSize" default:"1048576"`
	WaitingListMaxTTL   int64   `yaml:"waitingListMaxTTL" default:"60"`
	StreamCheckInterval int     `yaml:"streamCheckInterval" default:"60"`
	StreamTTL           float64 `yaml:"streamTTL" default:"300"`
}

type RetryConfig struct {
	MaxTimout     int64   `yaml:"maxTimout" default:"30"`
	BackoffFactor float64 `yaml:"backoffFactor" default:"0.2"`
}

type AckConfig struct {
	WaitingTimePercentOfRTO float64 `yaml:"waitingTimePercentOfRTO" default:"50"`
	EndpointLifeTime        int64   `yaml:"endpointLifeTime" default:"60"`
}

// AuthenticationConfig authentication config
type AuthenticationConfig struct {
	Key string `yaml:"key" default:""`
}

// User system user
type User struct {
	Name     string `yaml:"name"`
	Password string `yaml:"password"`
}

// TracingConfig tracing configuration
type TracingConfig struct {
	Socket       bool `yaml:"socket,omitempty" default:"false"`
	Interface    bool `yaml:"interface,omitempty" default:"false"`
	StreamMerger bool `yaml:"streamMerger,omitempty" default:"false"`
	Retry        bool `yaml:"retry,omitempty" default:"false"`
	Ack          bool `yaml:"ack,omitempty" default:"false"`
}

// ProfilerConfig pprof configuration
type ProfilerConfig struct {
	Enabled *bool  `yaml:"enabled" default:"false"`
	Host    string `yaml:"host" default:"localhost"`
	Port    int    `yaml:"port" default:"8888"`
}

// ApplicationConfig base application params
type ApplicationConfig struct {
	Homepage        string `yaml:"homepage" default:"resources/index.html"`
	HomepageWin     string `yaml:"homepageWin" default:"../index.html"`
	IconsPath       string `yaml:"iconsPath" default:"resources/icons"`
	AppIconLinux    string `yaml:"appIconLinux" default:"app.png"`
	AppIconDarwin   string `yaml:"appIconDarwin" default:"tapir.icns"`
	AppIconWindows  string `yaml:"appIconWindows" default:"app.ico"`
	TrayIconLinux   string `yaml:"trayIconLinux" default:"tray.png"`
	TrayIconDarwin  string `yaml:"trayIconDarwin" default:"tray24.png"`
	TrayIconWindows string `yaml:"trayIconWindows" default:"tray.ico"`
}

type GUISettings struct {
	WindowWidth          int   `yaml:"windowWidth" default:"200"`
	WindowHeight         int   `yaml:"windowHeight" default:"300"`
	WindowX              int   `yaml:"windowX" default:"600"`
	WindowY              int   `yaml:"windowY" default:"100"`
	ShutdownClientOnExit *bool `yaml:"shutdownClientOnExit" default:"true"`
}

type ClientIPCSettings struct {
	GrpcHost string `yaml:"grpcHost" default:"localhost"`
	GrpcPort int    `yaml:"grpcPort" default:"1977"`
}

type ClientRunnerSettings struct {
	CmdLinux   string `yaml:"cmdLinux" default:"pkexec {{ .client }} -config {{ .config }} &"`
	CmdDarwin  string `yaml:"cmdDarwin" default:"osascript -e \"do shell script \\\"{{ .client }}  -config {{ .config }} &>/dev/null &\\\" with administrator privileges\""`
	CmdWindows string `yaml:"cmdWindows" default:"powershell.exe Start-Process {{ .client }} -Verb runAs -WindowStyle Hidden"`
}

type StatisticConfig struct {
	Interval int `yaml:"interval" default:"1000"`
}

// RestConfig REST server configuration
type RestConfig struct {
	Enabled *bool  `yaml:"enabled" default:"false"`
	Host    string `yaml:"host" default:""`
	Port    int    `yaml:"port" default:"8877"`
}

func (c NetworkConfig) MaxPorts() int {
	ports := c.PortMax - c.PortMin + 1
	if *c.UseTCP && *c.UseUDP {
		ports *= 2
	}
	return int(ports)
}

func (c NetworkConfig) UseStreamMerger() bool {
	return c.PortMin < c.PortMax || *c.UseUDP
}

func (c NetworkConfig) GetConnectionProtocol() Protocol {
	proto := make([]Protocol, 0, 2)
	if *c.UseTCP {
		proto = append(proto, ProtoTCP)
	}
	if *c.UseUDP {
		proto = append(proto, ProtoUDP)
	}
	return proto[rand.Intn(len(proto))]
}

func (c *ServerConfig) Validate() error {
	return validation.ValidateStruct(c,
		validation.Field(&c.ServerHost, validation.Required, is.Host),
	)
}

func (c *ClientConfig) Validate() error {
	// TODO
	return nil
}

func (c *ClientConfig) Normalize() {
	c.System.ClientID = structs.If(c.System.ClientID == "", uuid.New().String(), c.System.ClientID)
	c.System.Shell = structs.If(c.System.Shell == "", GetSystemShell(), c.System.Shell)
}

func (c *ClientConfig) MaxPorts() int {
	var maxPorts int
	for _, c := range c.Connections {
		maxPorts += c.Server.MaxPorts()
	}
	return maxPorts
}

func (c *ClientConfig) Marshal() ([]byte, error) {
	return json.Marshal(c)
}

func (c *ClientConfig) Unmarshal(data []byte) (*ClientConfig, error) {
	cfg := &ClientConfig{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return c, err
	}
	return cfg, nil
}

func (c *ClientConnection) Unmarshal(data []byte) error {
	if err := yaml.Unmarshal(data, c); err != nil {
		return err
	}
	return nil
}

func (c ClientRunnerSettings) Get() string {
	switch runtime.GOOS {
	case "linux":
		return c.CmdLinux
	case "darwin":
		return c.CmdDarwin
	case "windows":
		return c.CmdWindows
	}
	panic("your OS is not supported")
}

func (c NetworkConfig) Clone() *NetworkConfig {
	clone, err := jsonClone(&c)
	if err != nil {
		panic(err)
	}
	return clone
}

func (c TunnelConfig) Clone() *TunnelConfig {
	clone, err := jsonClone(&c)
	if err != nil {
		panic(err)
	}
	return clone
}

func (c AuthenticationConfig) Clone() *AuthenticationConfig {
	clone, err := jsonClone(&c)
	if err != nil {
		panic(err)
	}
	return clone
}

func jsonClone[T any](in *T) (*T, error) {
	orig, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	out := new(T)
	if err = json.Unmarshal(orig, out); err != nil {
		return nil, err
	}
	return out, nil
}

func GetSystemShell() string {
	switch runtime.GOOS {
	case "linux":
		return "bash -c"
	case "darwin":
		return "zsh -c"
	case "windows":
		return "powershell.exe -nologo -noninteractive -windowStyle hidden -nologo -noninteractive"
	}
	panic("your OS is not supported")
}
