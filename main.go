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
const applicationVersion string = "v0.2.6.3"

var (
	myBridge     *huego.Bridge
	myBridgeID   string
	lightID      int
	loadedLights []huego.Light
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
	flag.Bool("bridgeconfig", false, "Show bridge configuration")

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

	if viper.IsSet("action") {
		if checkAction(viper.GetString("action")) {
			// action is good
			action = strings.ToLower(viper.GetString("action"))
			fmt.Printf("ACTION: \"--action %s\" is valid\n", action)
		} else {
			// action is bad
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
		os.Exit(0)
	}

	if viper.GetBool("showusers") {
		displayUsers(myBridge)
		os.Exit(0)
	}

	if viper.IsSet("bridgeconfig") {
		displayBridgeConfig()
		os.Exit(0)
	}

	// load up all the lights from bridge
	loadLights()

	if !areLightsLoaded() {
		fmt.Println("ERROR: No lights found")
		os.Exit(1)
	}

	if viper.IsSet("light") {
		var lighterr error
		lightID, lighterr = strconv.Atoi(viper.GetString("light"))
		if lighterr != nil {
			getid, foundLightID := getLightIDFromName(viper.GetString("light"))

			if foundLightID {
				//fmt.Printf("Matched light name \"%s\" to lightid %d\n", viper.GetString("light"), getid)
				lightID = getid
			} else {
				fmt.Printf("ERROR: \"--light %s\" is not a valid light name or light id\n", viper.GetString("light"))
				os.Exit(1)
			}
		}
	} /* else {
		fmt.Println("no light set")
	} */

	if viper.IsSet("list") || viper.IsSet("listall") {
		listLights()
	}

	if action != "" {
		if checkLightValid(lightID) {
			doAction()
		} else {
			// tidy
			fmt.Println("ERROR: light not found")
			os.Exit(1)
		}
	}
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
      --bridgeconfig        Show bridge configuration
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

// display list of valid actions
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

// display light information
func listLights() {

	if !areLightsLoaded() {
		fmt.Printf("ERROR: No lights to display")
	}

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

	for _, eachlight := range loadedLights {
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

// display bridge connection information
func displayBridge(thisBridge *huego.Bridge) {
	const padding = 1
	w := tabwriter.NewWriter(os.Stdout, 0, 2, padding, ' ', 0)
	fmt.Fprintf(w, "%-15s\t%s\t%s\t\n", "Host", "BridgeID", "User")
	fmt.Fprintf(w, "%-15s\t%s\t%s\t\n", "---------------", "--------", "----")
	fmt.Fprintf(w, "%-15s\t%s\t%s\t\n", thisBridge.Host, myBridgeID, thisBridge.User)
	w.Flush()
}

// display a list of all users/whitelists
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

// runs actions
func doAction() {
	fmt.Printf("Doing action: %s\n", action)

	if !areLightsLoaded() {
		fmt.Printf("ERROR: No lights to display")
	}

	// turn light on and off
	if strings.EqualFold(action, "on") || strings.EqualFold(action, "off") {
		light, err := myBridge.GetLight(lightID)
		if err != nil {
			// tidy
			panic(err)
		}

		if strings.EqualFold(action, "on") {
			light.On()
		} else {
			light.Off()
		}
	}

	// check status of light
	if strings.EqualFold(action, "status") {
		light, err := myBridge.GetLight(lightID)
		if err != nil {
			// tidy
			panic(err)
		}

		lightstate := "off"
		if light.IsOn() {
			lightstate = "on"
		}

		fmt.Printf("Light: \"%s\" is %s\n", light.Name, lightstate)
	}
}

// display all configuration of the bridge
func displayBridgeConfig() {
	myconfig, err := myBridge.GetConfig()
	if err != nil {
		// tidy
		panic(err)
	}

	const padding = 1
	w := tabwriter.NewWriter(os.Stdout, 0, 2, padding, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t\n", "Setting", "Configuration")
	fmt.Fprintf(w, "%s\t%s\t\n", "-------", "-------------")
	fmt.Fprintf(w, "%s\t%s\t\n", "Name", myconfig.Name)
	fmt.Fprintf(w, "%s\t%s\t\n", "BridgeID", myconfig.BridgeID)
	fmt.Fprintf(w, "%s\t%s\t\n", "ModelID", myconfig.ModelID)
	fmt.Fprintf(w, "%s\t%d\t\n", "ZigbeeChannel", myconfig.ZigbeeChannel)
	fmt.Fprintf(w, "%s\t%t\t\n", "FactoryNew", myconfig.FactoryNew)
	fmt.Fprintf(w, "%s\t%s\t\n", "ReplacesBridgeID", myconfig.ReplacesBridgeID)
	fmt.Fprintf(w, "%s\t%s\t\n", "DatastoreVersion", myconfig.DatastoreVersion)
	fmt.Fprintf(w, "%s\t%s\t\n", "StarterKitID", myconfig.StarterKitID)

	fmt.Fprintf(w, "%s\t%s\t\n", "InternetService.Internet", myconfig.InternetService.Internet)
	fmt.Fprintf(w, "%s\t%s\t\n", "InternetService.RemoteAccess", myconfig.InternetService.RemoteAccess)
	fmt.Fprintf(w, "%s\t%s\t\n", "InternetService.Time", myconfig.InternetService.Time)
	fmt.Fprintf(w, "%s\t%s\t\n", "InternetService.SwUpdate", myconfig.InternetService.SwUpdate)

	fmt.Fprintf(w, "%s\t%s\t\n", "SwUpdate2.Bridge.State", myconfig.SwUpdate2.Bridge.State)
	fmt.Fprintf(w, "%s\t%s\t\n", "SwUpdate2.Bridge.LastInstall", myconfig.SwUpdate2.Bridge.LastInstall)
	fmt.Fprintf(w, "%s\t%t\t\n", "SwUpdate2.CheckForUpdate", myconfig.SwUpdate2.CheckForUpdate)
	fmt.Fprintf(w, "%s\t%s\t\n", "SwUpdate2.State", myconfig.SwUpdate2.State)
	fmt.Fprintf(w, "%s\t%t\t\n", "SwUpdate2.Install", myconfig.SwUpdate2.Install)
	fmt.Fprintf(w, "%s\t%t\t\n", "SwUpdate2.AutoInstall.On", myconfig.SwUpdate2.AutoInstall.On)
	fmt.Fprintf(w, "%s\t%s\t\n", "SwUpdate2.AutoInstall.UpdateTime", myconfig.SwUpdate2.AutoInstall.UpdateTime)
	fmt.Fprintf(w, "%s\t%s\t\n", "SwUpdate2.LastChange", myconfig.SwUpdate2.LastChange)
	fmt.Fprintf(w, "%s\t%s\t\n", "SwUpdate2.LastInstall", myconfig.SwUpdate2.LastInstall)

	fmt.Fprintf(w, "%s\t%s\t\n", "APIVersion", myconfig.APIVersion)
	fmt.Fprintf(w, "%s\t%s\t\n", "SwVersion", myconfig.SwVersion)

	// WhitelistMap has the same contents as []Whitelist so can be ignored
	// fmt.Fprintf(w, "%s\t%s\t\n", "WhitelistMap", myconfig.WhitelistMap)

	// sort the whitelist/users alphabetically by name
	sort.SliceStable(myconfig.Whitelist, func(i, j int) bool {
		return myconfig.Whitelist[i].Name < myconfig.Whitelist[j].Name
	})

	for i, key := range myconfig.Whitelist {
		fmt.Fprintf(w, "%s%d%s\t%s\t\n", "Whitelist.", i, ".Name", key.Name)
		fmt.Fprintf(w, "%s%d%s\t%s\t\n", "Whitelist.", i, ".Username", key.Username)
		fmt.Fprintf(w, "%s%d%s\t%s\t\n", "Whitelist.", i, ".CreateDate", key.CreateDate)
		fmt.Fprintf(w, "%s%d%s\t%s\t\n", "Whitelist.", i, ".LastUseDate", key.LastUseDate)
		fmt.Fprintf(w, "%s%d%s\t%s\t\n", "Whitelist.", i, ".ClientKey", key.ClientKey)
	}

	fmt.Fprintf(w, "%s\t%t\t\n", "PortalState.SignedOn", myconfig.PortalState.SignedOn)
	fmt.Fprintf(w, "%s\t%t\t\n", "PortalState.Incoming", myconfig.PortalState.Incoming)
	fmt.Fprintf(w, "%s\t%t\t\n", "PortalState.Outgoing", myconfig.PortalState.Outgoing)
	fmt.Fprintf(w, "%s\t%s\t\n", "PortalState.Communication", myconfig.PortalState.Communication)

	fmt.Fprintf(w, "%s\t%s\t\n", "Network.IPAddress", myconfig.IPAddress)
	fmt.Fprintf(w, "%s\t%s\t\n", "Network.Mac", myconfig.Mac)
	fmt.Fprintf(w, "%s\t%s\t\n", "Network.NetMask", myconfig.NetMask)
	fmt.Fprintf(w, "%s\t%s\t\n", "Network.Gateway", myconfig.Gateway)
	fmt.Fprintf(w, "%s\t%t\t\n", "Network.DHCP", myconfig.Dhcp)
	fmt.Fprintf(w, "%s\t%s\t\n", "Network.ProxyAddress", myconfig.ProxyAddress)
	fmt.Fprintf(w, "%s\t%d\t\n", "Network.ProxyPort", myconfig.ProxyPort)

	fmt.Fprintf(w, "%s\t%t\t\n", "LinkButton", myconfig.LinkButton)

	fmt.Fprintf(w, "%s\t%s\t\n", "Time.UTC", myconfig.UTC)
	fmt.Fprintf(w, "%s\t%s\t\n", "Time.LocalTime", myconfig.LocalTime)
	fmt.Fprintf(w, "%s\t%s\t\n", "Time.TimeZone", myconfig.TimeZone)

	w.Flush()
}

// check if a lightID is valid
func checkLightValid(findLightID int) bool {
	if !areLightsLoaded() {
		fmt.Printf("ERROR: No lights found")
	}

	for _, eachlight := range loadedLights {
		if eachlight.ID == findLightID {
			return true
		}
	}

	return false
}

// loads lights from the bridge preventing multiple uneccessary calls to bridge
func loadLights() {
	var errload error
	lights, errload := myBridge.GetLights()
	if errload != nil {
		fmt.Println("ERROR: Could not load lights from bridge")
		os.Exit(1)
	}

	// if no lights were found
	if len(lights) < 1 {
		fmt.Println("ERROR: No lights found on bridge")
		os.Exit(1)
	}

	fmt.Printf("Found %d lights\n", len(lights))

	// sorting lights by ID
	sort.SliceStable(lights, func(i, j int) bool {
		return lights[i].ID < lights[j].ID
	})

	// copy in to global variable so that other functions can use the loaded lights list
	loadedLights = lights
}

// find a lightID when given the name of a light
func getLightIDFromName(lightName string) (int, bool) {
	if !areLightsLoaded() {
		fmt.Printf("ERROR: No lights to found")
	}

	for _, eachlight := range loadedLights {
		if strings.EqualFold(strings.ToLower(eachlight.Name), strings.ToLower(lightName)) {
			return eachlight.ID, true
		}
	}
	return 0, false
}

// have lights been loaded?
func areLightsLoaded() bool {
	if len(loadedLights) > 0 {
		return true
	}
	return false
}
