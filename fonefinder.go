package fonefinder

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/schema"
	"github.com/rickt/slack-appengine"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"net/http"
	"os"
	"sort"
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
		log.Criticalf(ctx, "ERROR decodeslackrequest(): token received from slack hook.Token=%s does not match expected token env.Token=%s!", hook.Token, env.Token)
		return errors.New("error, token received from slack does not match expected token!")
	}
	return nil
}

// no main() since this is an appengine app
func init() {
	// setup the url handler
	http.HandleFunc("/slack", slackhandler)
}

// search user & group lists
func searchforusers(ctx context.Context, name string) (slack.UserData, slack.UserGroupData, error) {
	if env.UserToken == "" {
		return nil, nil, errors.New("error, slack user token is invalid!")
	}
	// establish slack connection
	sl := slack.New(env.UserToken, ctx)
	// get the user list
	users, err := sl.UsersList()
	if err != nil {
		log.Errorf(ctx, "ERROR error retrieving userlist, err=%s", err)
	}
	// search the user list
	var searchedusers []*slack.User
	for _, user := range users {
		// remove bots/non-real users
		if strings.Contains(user.Profile.Email, "@") {
			// check username, first & last names
			if ciContains(user.Name, name) || ciContains(user.Profile.RealName, name) || ciContains(user.Profile.FirstName, name) || ciContains(user.Profile.LastName, name) {
				searchedusers = append(searchedusers, user)
				if env.Debug {
					log.Debugf(ctx, "DEBUG   user: Id=%s, Name=%s, RealName=%s, Phone=%s", user.Id, user.Name, user.Profile.RealName, user.Profile.Phone)
				}
			}
		}
	}
	// search the group list
	usergroups, err := sl.UserGroupsList()
	if err != nil {
		log.Errorf(ctx, "error retrieving usergrouplist, err=%s", err)
	}
	var searchedgroups []*slack.UserGroup
	for _, group := range usergroups {
		if ciContains(group.Handle, name) || ciContains(group.Description, name) || ciContains(group.Name, name) {
			searchedgroups = append(searchedgroups, group)
			if env.Debug {
				log.Debugf(ctx, "DEBUG   group: Id=%s, Name=%s, Handle=%s", group.ID, group.Name, group.Handle)
			}
		}
	}
	if env.Debug {
		log.Debugf(ctx, "DEBUG searchforusers(): len(searchedusers)=%v, len(usergroups)=%v", len(searchedusers), len(usergroups))
		log.Debugf(ctx, "DEBUG searchforusers(): searchedusers=%v", searchedusers)
		log.Debugf(ctx, "DEBUG searchforusers(): searchedgroups=%v", searchedgroups)
	}
	return searchedusers, searchedgroups, nil
}

// helper function to reply back to slack
func sendit(ctx context.Context, w http.ResponseWriter, textpayload string) {
	// build the slack payload
	payload := Payload{
		// ResponseType: "in_channel",
		Text: textpayload,
	}
	js, _ := json.Marshal(payload)
	if env.Debug {
		log.Debugf(ctx, "DEBUG sendit(): sent payload=%v", payload)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
	return
}

// func that handles the POST to /slack from slack
func slackhandler(w http.ResponseWriter, r *http.Request) {
	// get runtime options from the app.yaml
	isdebug := os.Getenv("DEBUG")
	if isdebug == "true" {
		env.Debug = true
	}
	// with appengine, we can't query environment variables until after init() completes. sucks.jpg
	env.Team = os.Getenv("SLACK_TEAM")
	env.Token = os.Getenv("SLACK_TOKEN")
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
		sendit(ctx, w, "unauthenticated request, tsk tsk!")
		return
	}
	if env.Debug {
		log.Debugf(ctx, "DEBUG slackhandler(): hook.Text=%v, hook.Token=%s", hook.Text, hook.Token)
	}
	// if the search term is too short, drop it
	if len(hook.Text) < 2 {
		if env.Debug {
			log.Debugf(ctx, fmt.Sprintf("DEBUG slackhandler(): search string too short! len=%d", len(hook.Text)))
		}
		sendit(ctx, w, fmt.Sprintf("I'm sorry, but your search phrase \"%s\" is too short! :confused:", hook.Text))
		return
	}
	// search the slack team directory
	var usergrouplist slack.UserGroupData
	var userlist slack.UserData
	userlist, usergrouplist, err = searchforusers(ctx, hook.Text)
	if env.Debug {
		log.Debugf(ctx, "DEBUG slackhandler(): userlist=%v", userlist)
		log.Debugf(ctx, "DEBUG slackhandler(): usergrouplist=%v", usergrouplist)
	}
	// build the response
	var phone, response string
	// if there's more than 1 user in the userlist, we have something to show
	if len(userlist) > 0 || len(usergrouplist) > 0 {
		// go through the userlist first, check for blank phone number fields
		if len(userlist) > 0 {
			response = response + fmt.Sprintf("*Users:*\n")
			// sort the userlist and lets go through it
			sort.Sort(userlist)
			for _, user := range userlist {
				// if a user doesn't have a phone number, UNLIST them
				if user.Profile.Phone == "" {
					phone = "UNLISTED"
				} else {
					phone = user.Profile.Phone
				}
				// build up the user data in our response 1 line at a time
				response = response + fmt.Sprintf("%s %s: :dir_phone: %s :dir_email: <mailto:%s|%s> :slack: <@%s|%s>\n", user.Profile.FirstName, user.Profile.LastName, phone, user.Profile.Email, user.Profile.Email, user.Id, user.Name)
			}
		}
		// now go through the usergroup list
		if len(usergrouplist) > 0 {
			response = response + fmt.Sprintf("*Groups:*\n")
			// sort the grouplist and lets go through it
			sort.Sort(usergrouplist)
			for _, usergroup := range usergrouplist {
				// build up the group data in our response 1 line at a time
				response = response + fmt.Sprintf("%s :slack: @%s\n", usergroup.Name, usergroup.Handle)
			}
		}
	} else {
		// if there's less than 1 user or usergroup in the list, there's nothing to show
		response = fmt.Sprintf("I'm sorry, but I was not able to find a user or group using your search term \"%s\"! :confused:", hook.Text)
	}
	sendit(ctx, w, response)
	return
}

// EOF
