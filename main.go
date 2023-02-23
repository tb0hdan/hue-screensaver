package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/amimof/huego"
	"gopkg.in/ini.v1"
)

type Config struct {
	cfg           *ini.File
	User          string
	Address       string
	HuePlayID     []string
	ManageNumlock bool
	Debug         bool
}

func (c *Config) String() string {
	return fmt.Sprintf(
		"User: %s, Address: %s, huePlayID: %s, ManageNumlock: %v, Debug: %v",
		c.User, c.Address, c.HuePlayID, c.ManageNumlock, c.Debug,
	)
}

func (c *Config) getSection(name string) *ini.Section {
	return c.cfg.Section(name)
}

func (c *Config) getStringKey(name string, section *ini.Section) string {
	return section.Key(name).String()
}

func (c *Config) getBoolKey(name string, section *ini.Section) bool {
	result, err := section.Key(name).Bool()
	if err != nil {
		return false
	}
	return result
}

func (c *Config) Load(cfgFile string) (err error) {
	c.cfg, err = ini.Load(cfgFile)
	if err != nil {
		return err
	}
	// Hue
	section := c.getSection("Hue")
	c.User = c.getStringKey("User", section)
	c.Address = c.getStringKey("Address", section)
	c.HuePlayID = strings.Split(c.getStringKey("HuePlayID", section), ",")
	// Keyboard
	keyboardSection := c.getSection("Keyboard")
	c.ManageNumlock = c.getBoolKey("ManageNumlock", keyboardSection)
	// Default
	defaultSection := c.getSection("Default")
	c.Debug = c.getBoolKey("Debug", defaultSection)
	return nil
}

type HueMon struct {
	numlockStatus bool
	keyboardLock  *sync.Mutex
	huePlayID     []string
	// Public
	Cfg   *Config
	Debug bool
}

func NewHueMon(cfg *Config) *HueMon {
	return &HueMon{
		keyboardLock: &sync.Mutex{},
		huePlayID:    cfg.HuePlayID,
		Cfg:          cfg,
		Debug:        cfg.Debug,
	}
}

func (hm *HueMon) GetLights(bridge *huego.Bridge) ([]int, error) {
	l, err := bridge.GetLights()
	if err != nil {
		return nil, err
	}
	if hm.Debug {
		fmt.Println(l)
	}
	playLights := make([]int, 0)
	for _, light := range l {
		isPlayBar := false
		for _, hueLightID := range hm.huePlayID {
			if light.ModelID != hueLightID {
				continue
			}
			isPlayBar = true
		}
		if isPlayBar {
			playLights = append(playLights, light.ID)
		}
	}

	return playLights, nil
}

func (hm *HueMon) TurnOn(playLights []int, bridge *huego.Bridge) {
	for _, lightID := range playLights {
		light, _ := bridge.GetLight(lightID)
		if light.IsOn() {
			continue
		}
		light.On()
	}
}

func (hm *HueMon) NumlockOn() {
	if !hm.Cfg.ManageNumlock {
		log.Debug("Numlock support disabled by configuration")
		return
	}
	hm.keyboardLock.Lock()
	defer hm.keyboardLock.Unlock()
	if hm.numlockStatus {
		return
	}
	_, err := exec.Command("numlockx", "on").Output()
	if err != nil {
		log.Errorf("NumlockOn: %v\n", err)
		return
	}
	hm.numlockStatus = true
	log.Debug("Numlock turned on")
}

func (hm *HueMon) NumlockOff() {
	if !hm.Cfg.ManageNumlock {
		log.Debug("Numlock support disabled by configuration")
		return
	}
	hm.keyboardLock.Lock()
	defer hm.keyboardLock.Unlock()
	if !hm.numlockStatus {
		return
	}
	_, err := exec.Command("numlockx", "off").Output()
	if err != nil {
		log.Errorf("NumlockOff: %v\n", err)
		return
	}
	hm.numlockStatus = false
	log.Debug("Numlock turned off")
}

func (hm *HueMon) TurnOff(playLights []int, bridge *huego.Bridge) {
	for _, lightID := range playLights {
		light, _ := bridge.GetLight(lightID)
		if !light.IsOn() {
			continue
		}
		light.Off()
	}
}

func (hm *HueMon) Watch(playLights []int, bridge *huego.Bridge) {
	out, err := exec.Command("xscreensaver-command", "-time").Output()
	if err != nil {
		panic(err)
	}
	result := string(out)
	//
	reg := regexp.MustCompile(`^.+\: screen`)
	result = reg.ReplaceAllString(result, ``)
	//
	reg = regexp.MustCompile(` since.*`)
	result = reg.ReplaceAllString(result, ``)
	//
	state := strings.TrimSpace(result)
	log.Println(state)
	switch state {
	case "blanked", "locked":
		hm.TurnOff(playLights, bridge)
		hm.NumlockOff()
	case "non-blanked":
		hm.TurnOn(playLights, bridge)
		hm.NumlockOn()
	default:
		log.Printf("Got unknown state: %s", state)
	}
}

func Discover(hostname string) {
	bridge, err := huego.Discover()
	if err != nil {
		panic(err)
	}

	user, err := bridge.CreateUser(hostname) // Link button needs to be pressed
	if err != nil {
		panic(err)
	}
	bridge = bridge.Login(user)
	log.Println(bridge, user)
}

func main() {
	cmd := flag.String("command", "", "On/Off etc")
	cfgFile := flag.String("cfg", "huemon.ini", "Path to configuration ini. Defaults to huemon.ini")
	discover := flag.Bool("discover", false, "Discover Hue bridges")
	hostname := flag.String(
		"hostname",
		func() string { h, _ := os.Hostname(); return h }(),
		"Hostname used for discovery, defaults to system",
	)
	flag.Parse()
	if *discover {
		Discover(*hostname)
		return
	}
	cfg := &Config{}
	err := cfg.Load(*cfgFile)
	if err != nil {
		panic(err)
	}
	log.SetLevel(log.InfoLevel)
	if cfg.Debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug mode enabled")
		log.Debug(cfg)
	}
	bridge := huego.New(cfg.Address, cfg.User)
	hueMon := NewHueMon(cfg)
	lights, err := hueMon.GetLights(bridge)
	if err != nil {
		panic(err)
	}
	fmt.Println("x", lights)
	// Turn on numlock on start
	hueMon.NumlockOn()
	//
	switch *cmd {
	case "on":
		hueMon.TurnOn(lights, bridge)
		hueMon.NumlockOn()
	case "off":
		hueMon.TurnOff(lights, bridge)
		hueMon.NumlockOff()
	case "watch":
		for {
			hueMon.Watch(lights, bridge)
			time.Sleep(3 * time.Second)
		}
	default:
		log.Printf("Unsupported command: `%s`\n", *cmd)
	}
}
