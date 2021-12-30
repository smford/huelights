package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/amimof/huego"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const applicationName string = "huelight"
const applicationVersion string = "v0.2.3"

var (
	myBridge     *huego.Bridge
	myBridgeID   string
	lightID      int
	action       string
	validActions = map[string]string{
		"on":     "Turn light on",
		"off":    "Turn light off",
		"status": "Show current state",
	}
)

func init() {
	// tidy
	flag.String("config", "config.yaml", "Configuration file: /path/to/file.yaml, default = ./config.yaml")
	flag.Bool("displayconfig", false, "Display configuration")
	flag.Bool("help", false, "Display help")
	flag.Bool("version", false, "Display version")
	flag.String("light", "", "Light ID")
	flag.String("action", "", "Action to do")
	flag.Bool("listall", false, "List all lights details")
	flag.Bool("list", false, "List lights")
	flag.Bool("showbridge", false, "Show bridge details")
	flag.Bool("showusers", false, "Show user list")

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)

	if viper.GetBool("help") {
		displayHelp()
		os.Exit(0)
	}

	if viper.GetBool("version") {
		fmt.Printf("%s %s\n", applicationName, applicationVersion)
		os.Exit(0)
	}

	configdir, configfile := filepath.Split(viper.GetString("config"))

	// set default configuration directory to current directory
	if configdir == "" {
		configdir = "."
	}

	viper.SetConfigType("yaml")
	viper.AddConfigPath(configdir)

	config := strings.TrimSuffix(configfile, ".yaml")
	config = strings.TrimSuffix(config, ".yml")

	viper.SetConfigName(config)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Fatal("Config file not found")
		} else {
			log.Fatal("Config file was found but another error was discovered: ", err)
		}
	}
}

func main() {
	if viper.GetBool("displayconfig") {
		displayConfig()
		os.Exit(0)
	}

	user := viper.GetString("hueuser")

	if viper.IsSet("light") {
		fmt.Printf("Light string: %s\n", viper.GetString("light"))
		lightID, err := strconv.Atoi(viper.GetString("light"))
		if err != nil {
			fmt.Printf("ERROR: \"--light %s\" is not valid\n", viper.GetString("light"))
			os.Exit(1)
		}
		fmt.Printf("Light number: %d\n", lightID)
	} else {
		fmt.Println("no light set")
	}

	if viper.IsSet("action") {
		if checkAction(viper.GetString("action")) {
			// action is good
			action = strings.ToLower(viper.GetString("action"))
			fmt.Printf("ACTION: \"--action %s\" is valid\n", action)
		} else {
			fmt.Printf("ERROR: \"--action %s\" is not valid\n", viper.GetString("action"))
			fmt.Println("Valid actions are:")
			listActions()
			os.Exit(1)
		}
	}

	var bridgeerr error
	myBridge, bridgeerr = huego.Discover()
	if bridgeerr != nil {
		// tidy
		panic(bridgeerr)
	}

	// store selected bridge ID because struct loses it once logged in
	myBridgeID = myBridge.ID

	// login in to bridge
	myBridge = myBridge.Login(user)

	if viper.GetBool("showbridge") {
		displayBridge(myBridge)
	}

	if viper.GetBool("showusers") {
		displayUsers(myBridge)
	}

	/*
		lights, err := myBridge.GetLights()
		if err != nil {
			panic(err)
		}
		fmt.Printf("Found %d lights\n", len(lights))

		// display all lights
		const padding = 1
		w := tabwriter.NewWriter(os.Stdout, 0, 2, padding, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t\n", "ID", "State", "Name", "Type", "ModelID", "Manufacturor", "UniqueID", "SwVersion", "SwConfigID", "ProductName")

		sort.SliceStable(lights, func(i, j int) bool {
			return lights[i].ID < lights[j].ID
		})

		for _, eachlight := range lights {
			status := ""
			if eachlight.State.On {
				status = "on"
			} else {
				status = "off"
			}

			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t\n", eachlight.ID, status, eachlight.Name, eachlight.Type, eachlight.ModelID, eachlight.ManufacturerName, eachlight.UniqueID, eachlight.SwVersion, eachlight.SwConfigID, eachlight.ProductName)

		}
		w.Flush()
	*/

	if viper.IsSet("list") || viper.IsSet("listall") {
		fmt.Println("====================")
		listLights()
	}

	doAction()
}

// displays configuration
func displayConfig() {
	allmysettings := viper.AllSettings()
	var keys []string
	for k := range allmysettings {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Println("CONFIG:", k, ":", allmysettings[k])
	}
}

// displays help information
func displayHelp() {
	// tidy
	message := `
      --config string       Configuration file: /path/to/file.yaml (default "./config.yaml")
      --displayconfig       Display configuration
      --help                Display help
      --version             Display version
      --list                List lights
      --listall             List all details about the lights
      --action              Do actions
      --showusers           List all user/whitelist details
      --showbridge          Show logged in bridge details
      --light               Select a light
`
	fmt.Println(applicationName + " " + applicationVersion)
	fmt.Println(message)
}

// checks if an action is valid
func checkAction(actionCheck string) bool {
	if _, ok := validActions[strings.ToLower(actionCheck)]; ok {
		return true
	} else {
		return false
	}
}

// prints list of valid actions
func listActions() {
	// sort the keys alphabetically to make better to display
	var sortedKeys []string
	for k := range validActions {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	const padding = 1
	w := tabwriter.NewWriter(os.Stdout, 0, 2, padding, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t\n", "Action", "Description")
	fmt.Fprintf(w, "%s\t%s\t\n", "------", "-----------")

	for _, k := range sortedKeys {
		fmt.Fprintf(w, "%s\t%s\t\n", k, validActions[k])
	}

	w.Flush()
}

func listLights() {
	lights, err := myBridge.GetLights()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Found %d lights\n", len(lights))

	// display all lights
	const padding = 1
	w := tabwriter.NewWriter(os.Stdout, 0, 2, padding, ' ', 0)
	if viper.GetBool("listall") {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t\n", "ID", "State", "Name", "Type", "ModelID", "Manufacturor", "UniqueID", "SwVersion", "SwConfigID", "ProductName")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t\n", "--", "-----", "----", "----", "-------", "------------", "--------", "---------", "----------", "-----------")
	} else {
		fmt.Fprintf(w, "%s\t%s\t%s\t\n", "ID", "State", "Name")
		fmt.Fprintf(w, "%s\t%s\t%s\t\n", "--", "-----", "----")
	}
	sort.SliceStable(lights, func(i, j int) bool {
		return lights[i].ID < lights[j].ID
	})

	for _, eachlight := range lights {
		status := ""
		if eachlight.State.On {
			status = "on"
		} else {
			status = "off"
		}

		if viper.GetBool("listall") {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t\n", eachlight.ID, status, eachlight.Name, eachlight.Type, eachlight.ModelID, eachlight.ManufacturerName, eachlight.UniqueID, eachlight.SwVersion, eachlight.SwConfigID, eachlight.ProductName)
		} else {
			fmt.Fprintf(w, "%d\t%s\t%s\t\n", eachlight.ID, status, eachlight.Name)
		}

	}
	w.Flush()
}

func displayBridge(thisBridge *huego.Bridge) {
	const padding = 1
	w := tabwriter.NewWriter(os.Stdout, 0, 2, padding, ' ', 0)
	fmt.Fprintf(w, "%-15s\t%s\t%s\t\n", "Host", "BridgeID", "User")
	fmt.Fprintf(w, "%-15s\t%s\t%s\t\n", "---------------", "--------", "----")
	fmt.Fprintf(w, "%-15s\t%s\t%s\t\n", thisBridge.Host, myBridgeID, thisBridge.User)
	w.Flush()
}

func displayUsers(thisBridge *huego.Bridge) {
	allusers, err := thisBridge.GetUsers()
	if err != nil {
		// tidy
		panic(err)
	}

	const padding = 1
	w := tabwriter.NewWriter(os.Stdout, 0, 2, padding, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t\n", "Name", "Username", "CreateDate", "LastUseDate", "ClientKey")
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t\n", "----", "--------", "----------", "-----------", "---------")

	// sort the users slice to make output consistent
	sort.SliceStable(allusers, func(i, j int) bool {
		return allusers[i].Name < allusers[j].Name
	})

	for _, eachuser := range allusers {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t\n", eachuser.Name, eachuser.Username, eachuser.CreateDate, eachuser.LastUseDate, eachuser.ClientKey)
	}

	w.Flush()

	fmt.Printf("\nNumber of users found: %d\n", len(allusers))
}

func doAction() {
	fmt.Println("doing action")
}
