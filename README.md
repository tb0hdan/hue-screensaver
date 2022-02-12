# hue-screensaver
Turn on Hue lamps on screensaver unlock, turn off on lock

## Build

`make`


## Initial configuration

1. `cp huemon.ini.example huemon.ini`
2. Press Hue bridge button
3. `./huemon -discover`
4. Save values to `huemon.ini`

## Running

`while :; do ./huemon -command watch; sleep 3; done`

## Enjoy!
