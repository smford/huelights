package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
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
const applicationVersion string = "v0.4"

var (
	myBridge     *huego.Bridge
	myBridgeID   string
	lightID      int
	loadedLights []huego.Light
	foundBridges []huego.Bridge
	action       string
	validActions = map[string]string{
		"on":         "Turn light on",
		"off":        "Turn light off",
		"status":     "Show current state",
		"hue":        "Set colour",
		"brightness": "Set Brightness",
	}
	colours = map[string]huelightColour{
		"red":   {float32(0.675), float32(0.322)},
		"green": {float32(0.4091), float32(0.518)},
		"blue":  {float32(0.167), float32(0.04)},
		"white": {float32(0.3227), float32(0.3290)},
	}
)

type huelightConfig struct {
	Bridge      string `yaml:"bridge"`
	Username    string `yaml:"username"`
	Application string `yaml:"application"`
}

type huelightColour struct {
	X float32 `yaml:"x"`
	Y float32 `yaml:"y"`
}

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
	flag.Bool("showusers", false, "Show whitelist/all users")
	flag.Bool("bridgeconfig", false, "Show bridge configuration")
	flag.String("createuser", "", "Creates a user")
	flag.String("deleteuser", "", "Deletes a user")
	flag.Bool("findbridges", false, "Searches network for Hue Bridges")
	flag.String("bridge", "", "Which bridge to use (IP Address)")
	flag.String("username", "", "Username to login to bridge")
	flag.Bool("makeconfig", false, "Make a configuration file")
	flag.String("value", "", "Value")
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

	if viper.GetBool("makeconfig") {
		fmt.Print("Do you want to create a config file? [y/n]: ")
		if yesNoPrompt() {
			setupConfig()
			os.Exit(0)
		} else {
			fmt.Println("did not want to setup a config file, exiting")
			os.Exit(2)
		}
	}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Printf("ERROR: Config file \"%s\" not found, exiting\n", viper.GetString("config"))
			os.Exit(1)
		} else {
			log.Fatal("Config file was found but another error was discovered: ", err)
		}
	}

	if viper.GetBool("displayconfig") {
		displayConfig()
		os.Exit(0)
	}

	if !viper.IsSet("bridge") {

		fmt.Println("no bridge set")
		os.Exit(1)
	}
}

func main() {
	if viper.IsSet("findbridges") {
		discoverBridges()
		printDiscoveredBridges()
		os.Exit(0)
	}

	user := viper.GetString("username")

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

	bridgeLogin(user)

	if viper.IsSet("createuser") {
		if !viper.IsSet("bridge") {
			fmt.Printf("\nWARN: Bridge has not been set, here is a list of discovered Hue bridges:\n\n")
			discoverBridges()
			printDiscoveredBridges()
			var userprompt string
			fmt.Printf("\nPlease type the IP address of the Hue bridge you wish to use: ")
			fmt.Scanln(&userprompt)

			if checkBridgeValid(userprompt) {
				fmt.Printf("Bridge found, using: %s\n", userprompt)
				//myBridge = getBridge(userprompt)
				// store selected bridge ID because struct loses it once logged in
				myBridgeID = myBridge.ID
			} else {
				fmt.Printf("Bridge not found, exiting: %s\n", userprompt)
				os.Exit(1)
			}

		}
		didmakeuser, username := createUser(viper.GetString("createuser"))
		if didmakeuser {
			fmt.Printf("Created User: %s\n", viper.GetString("createuser"))
			fmt.Printf("    Username: %s\n\n", username)
			fmt.Println("Hue uses the terms \"user\" and \"username\" in a confusing way.  User typically refer to an \"application\", whereas Username refers to Hue generated secret string used like a password or an API key.  This tool uses the Username when interacting with the Hue Bridge.")
			fmt.Println("\nCurrent whitelist/users are:")
			bridgeLogin(username)
			displayUsers(myBridge)
			os.Exit(0)
		} else {
			fmt.Printf("ERROR: could not create user: %s\n", viper.GetString("createuser"))
			os.Exit(1)
		}
	}

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

	if viper.IsSet("deleteuser") {
		fmt.Println("You can only delete a user via the Hue website at https://account.meethue.com/apps")
		os.Exit(0)
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
      --config string           Configuration file: /path/to/file.yaml (default "./config.yaml")
      --displayconfig           Display configuration
      --help                    Display help
      --version                 Display version
      --list                    List lights
      --listall                 List all details about the lights
      --action                  Do actions
      --showusers               List all user/whitelist details
      --showbridge              Show logged in bridge details
      --light                   Select a light
      --bridgeconfig            Show bridge configuration
      --createuser [username]   Creates a user
      --deleteuser              Deletes a user
      --findbridges             Discover Hue bridges on network
      --bridge                  Which bridge to use (IP Address)
      --username                Username to login to bridge
      --makeconfig              Make a configuration file
      --value                   Value to set
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

	//======
	if strings.EqualFold(action, "hue") {
		if !viper.IsSet("value") {
			fmt.Println("ERROR: you must also use --value when using action \"hue\"")
			os.Exit(1)
		}

		if !checkValue(viper.GetString("value")) {
			fmt.Printf("ERROR: value \"%s\" is not valid\n", viper.GetString("value"))
			os.Exit(0)
		}

		var newState huego.State

		//newState.Xy = uint16(checkedValue)
		//newState.Xy = []float32{float32(0.3227), float32(0.3290)} // white
		//newState.Xy = []float32{colours[strings.ToLower(viper.GetString("value")).X, colours[strings.ToLower(viper.GetString("value")).Y]}
		//newState.Xy = []float32{float32(0.4), float32(0.5)}
		fmt.Printf("colour: %s\nX: %f\nY: %f\n", viper.GetString("value"), colours[viper.GetString("value")].X, colours[viper.GetString("value")].Y)
		newState.Xy = []float32{
			colours[viper.GetString("value")].X,
			colours[viper.GetString("value")].Y,
		}
		newState.On = true
		//newState.Effect = "colorloop"
		//newState.Bri = 10

		fmt.Println("==================")
		fmt.Println(prettyPrint(newState))
		fmt.Println("==================")

		myResponse, err := myBridge.SetLightState(lightID, newState)
		if err != nil {
			// tidy
			panic(err)
		}

		fmt.Println("==================")
		fmt.Println(prettyPrint(myResponse))
		fmt.Println("==================")

	}

	//======
	if strings.EqualFold(action, "brightness") {
		if !viper.IsSet("value") {
			fmt.Println("ERROR: you must also use --value when using action \"brightness\"")
			os.Exit(1)
		}

		var newState huego.State

		newState.On = true

		newbright, brierr := strconv.Atoi(viper.GetString("value"))

		if brierr != nil {
			// tidy
			fmt.Printf("ERROR: brightness value \"%s\" is not valid\n", viper.GetString("value"))
			os.Exit(1)
		}

		if (newbright < 0) || (newbright > 100) {
			fmt.Printf("ERROR: Valid brightness values are 1 - 100 inclusive\n")
			os.Exit(1)
		}

		var calculateBrightness float32

		// why 254 rather than 256?  Because hue api maximum brightness is 254
		calculateBrightness = (254.0 / 100.0) * float32(newbright)

		//calculateBrightness = calculateBrightness * float32(newbright)
		//fmt.Printf("after 2: %f\n", calculateBrightness)

		newState.Bri = uint8(calculateBrightness)

		//myResponse, err := myBridge.SetLightState(lightID, newState)
		_, err := myBridge.SetLightState(lightID, newState)
		if err != nil {
			// tidy
			panic(err)
		}

		//fmt.Println(prettyPrint(myResponse))
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

// check if a user exists
func doesUserExist(checkuser string) bool {
	fmt.Println("starting doesuserexist")
	allusers, err := myBridge.GetUsers()
	if err != nil {
		// tidy
		panic(err)
	}

	prettyPrint(allusers)

	for _, eachuser := range allusers {
		if strings.EqualFold(strings.ToLower(eachuser.Name), checkuser) {
			fmt.Printf("Found user: %s\n", checkuser)
			return true
		}
	}

	fmt.Printf("NOT found user: %s\n", checkuser)
	return false
}

// creates a user/app/whitelist
func createUser(newuser string) (bool, string) {

	if doesUserExist(newuser) {
		// user already exists
		fmt.Println("ERROR: user already exists")
		return false, ""
	}

	// user doesn't exists so lets create one
	fmt.Println("creating user here")
	var userprompt string
	fmt.Println("To create the user you must first press the button on Hue Bridge.  Please press the button then return here and press the [return] key")
	fmt.Scanln(&userprompt)
	username, err := myBridge.CreateUser(newuser)

	if err == nil {
		return true, username
	}

	fmt.Println("==============")

	fmt.Println(prettyPrint(err))
	fmt.Println("==============")

	return false, ""
}

// pretty print a struct
func prettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

// login to the bridge
func bridgeLogin(loginas string) {
	myBridge = myBridge.Login(loginas)
}

// discover all bridges
func discoverBridges() {
	var brerr error
	foundBridges, brerr = huego.DiscoverAll()
	if brerr != nil {
		// tidy
		panic(brerr)
	}

	/*
		if len(foundBridges) < 1 {
			fmt.Println("ERROR: No Hue bridges found on network")
			os.Exit(1)
		}
		const padding = 1
		w := tabwriter.NewWriter(os.Stdout, 0, 2, padding, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t\n", "IP Address", "ID")
		fmt.Fprintf(w, "%s\t%s\t\n", "----------", "--")
		for _, eachbridge := range foundBridges {
			fmt.Fprintf(w, "%s\t%s\t\n", eachbridge.Host, eachbridge.ID)
		}
		w.Flush()
		fmt.Printf("\nFound %d bridges\n", len(foundBridges))
	*/
}

// checks if a bridge is valid
func checkBridgeValid(mybridge string) bool {
	if len(foundBridges) < 1 {
		return false
	}

	for _, eachbridge := range foundBridges {
		if strings.EqualFold(eachbridge.Host, mybridge) {
			return true
		}
	}

	return false
}

// returns a selected bridge, from IP address, after a DiscoverAll has populated the foundBridges variable
func getBridge(mybridge string) huego.Bridge {
	var returnbridge huego.Bridge
	for _, eachbridge := range foundBridges {
		fmt.Printf("pretty: %s\n", prettyPrint(eachbridge))
		if strings.EqualFold(eachbridge.Host, mybridge) {
			returnbridge = eachbridge
		}
	}
	return returnbridge
}

// sets up configuration
func setupConfig() {

	var myNewConfig huelightConfig

	var newConfigFile string

	if !viper.IsSet("config") {
		var userprompt string
		fmt.Println()
		fmt.Printf("The default configuration file %s looks for is \"config.yaml\" in the current directory.\n\nIf you choose a different name it will need to end in .yml or .yaml and always be passed to %s with the --config [filename] argument.\n", applicationName, applicationName)
		fmt.Print("Please choose a filename: ")
		fmt.Scanln(&userprompt)

		// fix: improve the checking of file
		if len(userprompt) < 4 {
			fmt.Println("filename too short, exiting")
			os.Exit(1)
		}

		newConfigFile = userprompt
	} else {
		// fix: check config file before blatting it
		newConfigFile = viper.GetString("config")
	}

	discoverBridges()

	if !viper.IsSet("bridge") {

		fmt.Println()
		printDiscoveredBridges()

		var userprompt string
		fmt.Print("\nPlease type the IP of bridge you want to use: ")
		fmt.Scanln(&userprompt)

		// fix: improve the checking of file

		myNewConfig.Bridge = userprompt

	} else {
		myNewConfig.Bridge = viper.GetString("bridge")
	}

	// check if bridge is valid
	if !checkBridgeValid(myNewConfig.Bridge) {
		fmt.Printf("WARN: Bridge \"%s\" is not valid, do you wish to continue [y/n]: ", myNewConfig.Bridge)
		if !yesNoPrompt() {
			os.Exit(1)
		}
	}

	if !viper.IsSet("username") {
		var userprompt string
		fmt.Print("Please type a username: ")
		fmt.Scanln(&userprompt)

		// fix: check username

		myNewConfig.Username = userprompt
	} else {
		myNewConfig.Username = viper.GetString("username")
	}

	myNewConfig.Application = applicationName

	fmt.Println("---------------")
	fmt.Printf("Config file: %s\n", newConfigFile)
	fmt.Printf("     Bridge: %s\n", myNewConfig.Bridge)
	fmt.Printf("   Username: %s\n", myNewConfig.Username)
	fmt.Printf("Application: %s\n", myNewConfig.Application)
	fmt.Println()

	fmt.Printf("Save this configuration to file \"%s\" [y/n]: ", newConfigFile)
	if yesNoPrompt() {
		fmt.Println("Saving configuration")

		yamlData, err := yaml.Marshal(&myNewConfig)

		if err != nil {
			fmt.Printf("ERROR: Cannot generate configuration. %v", err)
		}

		err = ioutil.WriteFile(newConfigFile, yamlData, 0644)
		if err != nil {
			fmt.Printf("ERROR: Unable to save into the file: %s\n", newConfigFile)
			fmt.Println(err)
			os.Exit(1)
		}

	} else {
		fmt.Println("\nWARN: Aborting config file save")
	}
}

// display found bridges
func printDiscoveredBridges() {
	if len(foundBridges) < 1 {
		fmt.Println("ERROR: No Hue bridges found on network")
		os.Exit(1)
	}
	const padding = 1
	w := tabwriter.NewWriter(os.Stdout, 0, 2, padding, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t\n", "IP Address", "ID")
	fmt.Fprintf(w, "%s\t%s\t\n", "----------", "--")
	for _, eachbridge := range foundBridges {
		fmt.Fprintf(w, "%s\t%s\t\n", eachbridge.Host, eachbridge.ID)
	}
	w.Flush()
	fmt.Printf("\nFound %d bridges\n", len(foundBridges))
}

// simple yes or no prompt, returns true if y or yes
func yesNoPrompt() bool {
	var userprompt string
	fmt.Scanln(&userprompt)
	if strings.EqualFold(userprompt, "y") || strings.EqualFold(userprompt, "yes") {
		return true
	}

	return false
}

// checks that a --value is valid and converts to an int where appropriate
func checkValue(value string) bool {

	if _, ok := colours[strings.ToLower(value)]; ok {
		return true
	}

	return false

}
