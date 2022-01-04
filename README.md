# huelights

A simple tool to control hue lights from the command line

## To do
- choose application name (user/whitelist name) and use Hue convention of applicaton#user
- check filenames
- check usernames (if possible) when making config
- when making config file and getting username, allow the creation of a user
- fix all "// fix"
- tidy all "// tidy"
- impliment better logging and output
- handle setting colour of lights
- handle when setting a colour for a light that doesn't support it
- handle colour gamuts depending on light type (https://github.com/benknight/hue-python-rgb-converter/blob/master/rgbxy/__init__.py and https://www.enigmaticdevices.com/philips-hue-lights-popular-xy-color-values/)
- add/remove/update a light
- list sensors
- add/remove/update a sensor

## Done
- print bridge details
- show users/whitelist
- turn lights on and off
- get status of light (on or off)
- display bridge configuration
- check if lightid is valid
- allow lightID or name to be passed with --light
- select which bridge
- create user
- generate and save a config file

## Abandoned
- delete user/whitelist: cannot be done via api, can only be done via https://account.meethue.com/apps
