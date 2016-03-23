# slack-fonefinder
##### overview
golang backend for a super-handy Slack /slash command that searches the team directory.

# ☎finder
installed as "/ff" in slack, takes a string argument as "name", searches the Slack Team Directory for it, returns the best match & phone number to the user.

written specifically to run in Google App Engine.  

##### notes
* this is written specifically for google app engine, hence no main() and  [github.com/rickt/slack-appengine](https://github.com/rickt/slack-appengine) requirement
* change values as appropriate in the environment variable section in your app.yaml

##### demo
this app is currently up & running at [fonefinderzu.appspot.com](http://fonefinderzu.appspot.com/slack)
