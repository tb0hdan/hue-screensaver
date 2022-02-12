package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/amimof/huego"
	"gopkg.in/ini.v1"
)

type Config struct {
	cfg       *ini.File
	User      string
	Address   string
	HuePlayID string
}

func (c *Config) getSection(name string) *ini.Section {
	return c.cfg.Section(name)
}

func (c *Config) getStringKey(name string, section *ini.Section) string {
	return section.Key(name).String()
}

func (c *Config) Load(cfgFile string) (err error) {
	c.cfg, err = ini.Load(cfgFile)
	if err != nil {
		return err
	}
	section := c.getSection("Hue")
	c.User = c.getStringKey("User", section)
	c.Address = c.getStringKey("Address", section)
	c.HuePlayID = c.getStringKey("HuePlayID", section)
	return nil
}

type HueMon struct {
	HuePlayID string
}

func (hm *HueMon) GetLights(bridge *huego.Bridge) ([]int, error) {
	l, err := bridge.GetLights()
	if err != nil {
		return nil, err
	}

	playLights := make([]int, 0)
	for _, light := range l {
		if light.ModelID != hm.HuePlayID {
			continue
		}
		playLights = append(playLights, light.ID)
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
	fmt.Println(state)
	switch state {
	case "blanked", "locked":
		hm.TurnOff(playLights, bridge)
	case "non-blanked":
		hm.TurnOn(playLights, bridge)
	default:
		fmt.Printf("Got unknown state: %s", state)
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
	fmt.Println(bridge, user)
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
	bridge := huego.New(cfg.Address, cfg.User)
	hueMon := &HueMon{HuePlayID: cfg.HuePlayID}
	lights, err := hueMon.GetLights(bridge)
	if err != nil {
		panic(err)
	}
	switch *cmd {
	case "on":
		hueMon.TurnOn(lights, bridge)
	case "off":
		hueMon.TurnOff(lights, bridge)
	case "watch":
		for {
			hueMon.Watch(lights, bridge)
			time.Sleep(3 * time.Second)
		}
	default:
		fmt.Printf("Unsupported command: %s", *cmd)
	}
}
