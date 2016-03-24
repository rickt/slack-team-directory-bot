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
	isdebug := os.Getenv("DEBUG")
	if isdebug == "true" {
		env.Debug = true
	}
	// w/appengine, we can't query environment variables until after init() completes. sucks.jpg
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
	// decode slack request
	err = decodeslackrequest(ctx, r, &hook)
	if err != nil {
		// unauthenticated request, handle it & error out
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
	// search the slack team directory
	userlist, err := searchforusers(ctx, hook.Text)
	if env.Debug {
		log.Debugf(ctx, "DEBUG slackhandler(): userlist=%v", userlist)
	}
	// build the response
	var phone, response string
	// if there's more than 1 user in the userlist, we have something to show
	if len(userlist) > 0 {
		// go through the userlist, check for blank phone number fields
		for _, user := range userlist {
			// if a user doesn't have a phone number, UNLIST them
			if user.Profile.Phone == "" {
				phone = "UNLISTED"
			} else {
				phone = user.Profile.Phone
			}
			// build up the data in our response 1 line at a time
			response = response + fmt.Sprintf("%s %s: :phone: %s :email: <mailto:%s|%s> :slack: <@%s|%s>\n", user.Profile.FirstName, user.Profile.LastName, phone, user.Profile.Email, user.Profile.Email, user.Id, user.Name)
		}
	} else {
		// if there's less than 1 user in the userlist, there's nothing to show
		response = fmt.Sprintf("Sorry, I was not able to find anyone using \"%s\"! :confused:", hook.Text)
	}
	// build the slack payload
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
	return
}

// func that decodes the slack request
func decodeslackrequest(ctx context.Context, r *http.Request, hook *slackRequest) error {
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
func searchforusers(ctx context.Context, name string) ([]*slack.User, error) {
	if env.UserToken == "" {
		return nil, errors.New("error, slack user token is invalid!")
	}
	// establish slack connection
	sl := slack.New(env.UserToken, ctx)
	// get the user list
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
