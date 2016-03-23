package fonefinder

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/schema"
	"github.com/rickt/slack-appengine"
	"golang.org/x/net/context"
	"google.golang.org/appengine"     // required to use the appengine logging package
	"google.golang.org/appengine/log" // appengine logging package
	"net/http"
	"os"
	"strings"
)

// vars
var (
	env envs
)

// helper function to do a case-insensitive search
func ciContains(a, b string) bool {
	return strings.Contains(strings.ToUpper(a), strings.ToUpper(b))
}

// no main() since this is an appengine app
func init() {
	// setup the url handler
	http.HandleFunc("/slack", slackhandler)
}

// func that handles the POST to /slack from slack
func slackhandler(w http.ResponseWriter, r *http.Request) {
	// get runtime options from the app.yaml
	// w/appengine, we can't query environment variables until after init() completes. sucks.jpg
	isdebug := os.Getenv("DEBUG")
	if isdebug == "true" {
		env.Debug = true
	}
	env.Token = os.Getenv("SLACK_TOKEN")
	env.TriggerWord = os.Getenv("SLACK_TRIGGERWORD")
	env.UserToken = os.Getenv("SLACK_USER_TOKEN")
	// create a google appengine context
	ctx := appengine.NewContext(r)
	hook := slackRequest{}
	// get the data from the POST from slack
	err := r.ParseForm()
	if err != nil {
		log.Errorf(ctx, "ERROR slackhandler(): parsing form error! err=%s", err)
		http.NotFound(w, r)
		return
	}
	defer r.Body.Close()
	log.Infof(ctx, "DEBUG slackhandler(): env.Token=%s, env.TriggerWord=%s, env.UserToken=%s, env.Debug=%v", env.Token, env.TriggerWord, env.UserToken, env.Debug)
	// decode slack request
	err = decodeslackrequest(r, &hook)
	if err != nil {
		log.Errorf(ctx, "ERROR slackhandler(): error decoding request from slack!! err=%v", err)
		payload := Payload{
			Text: "unauthenticated request, tsk tsk!",
		}
		js, _ := json.Marshal(payload)
		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
		return
	}
	if env.Debug {
		log.Debugf(ctx, "DEBUG slackhandler(): hook.Text=%v, hook.Token=%s, hook.TriggerWord=%v", hook.Text, hook.Token, hook.TriggerWord)
	}
	// search for the user
	userlist, err := searchforusers(hook.Text, ctx)
	if env.Debug {
		log.Debugf(ctx, "DEBUG slackhandler(): userlist=%v", userlist)
	}
	// build the response
	var phone, response string
	for _, user := range userlist {
		if user.Profile.Phone == "" {
			phone = "UNLISTED"
		} else {
			phone = user.Profile.Phone
		}
		response = response + fmt.Sprintf("%s %s: :phone: %s :email: <mailto:%s|%s> :slack: <@%s|%s>\n", user.Profile.FirstName, user.Profile.LastName, phone, user.Profile.Email, user.Profile.Email, user.Id, user.Name)
	}
	payload := Payload{
		// ResponseType: "in_channel",
		Text: response,
	}
	if env.Debug {
		log.Debugf(ctx, "DEBUG slackhandler(): payload=%v", payload)
	}
	// json it up & send it
	js, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

// func that decodes the slack request
func decodeslackrequest(r *http.Request, hook *slackRequest) error {
	// create a google appengine context
	ctx := appengine.NewContext(r)
	// create a decoder
	decoder := schema.NewDecoder()
	// decode
	err := decoder.Decode(hook, r.PostForm)
	if err != nil {
		log.Warningf(ctx, "DEBUG decodeslackrequest(): decode error! err=%s, hook=%v", err, hook)
	}
	// is this a valid request from slack? check for the token
	if env.Token != hook.Token {
		log.Errorf(ctx, "ERROR decodeslackrequest(): token received from slack hook.Token=%s does not match expected token env.Token=%s!", hook.Token, env.Token)
		return errors.New("error, token received from slack does not match expected token!")
	}
	return nil
}

// search for the user
func searchforusers(name string, ctx context.Context) ([]*slack.User, error) {
	if env.UserToken == "" {
		return nil, errors.New("error, slack user token is invalid!")
	}
	// establish slack connection
	sl := slack.New(env.UserToken, ctx)
	users, err := sl.UsersList()
	if err != nil {
		log.Errorf(ctx, "error retrieving userlist, err=%s", err)
		return nil, err
	}
	// find the name we were given
	var searchedusers []*slack.User
	for _, user := range users {
		// remove bots/non-real users
		if strings.Contains(user.Profile.Email, "@") {
			// check username, first & last names
			if ciContains(user.Name, name) || ciContains(user.Profile.RealName, name) || ciContains(user.Profile.FirstName, name) || ciContains(user.Profile.LastName, name) {
				searchedusers = append(searchedusers, user)
				log.Debugf(ctx, "DEBUG   Id=%s, Name=%s, RealName=%s, Phone=%s", user.Id, user.Name, user.Profile.RealName, user.Profile.Phone)
			}
		}
	}
	return searchedusers, nil
}

// EOF
