# slack-fonefinder
##### overview
# ☎finder
go backend for a simple slack phone query slash command. installed as "/ff" in slack, takes a string as "name", searches for it, returns the best match & phone number to the user.

written specifically to run in Google App Engine.  

##### notes
* this is written specifically for google app engine, hence no main() and  [github.com/rickt/slack-appengine](https://github.com/rickt/slack-appengine) requirement
* change values as appropriate in the environment variable section in your app.yaml

##### demo
this app is currently up & running at [fonefinderzu.appspot.com](http://fonefinderzu.appspot.com/slack)
