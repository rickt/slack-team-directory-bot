package slackteamdirectorybot

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
	// environment variables
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
		log.Criticalf(ctx, "ERROR decodeslackrequest(): token received from slack hook.Token does not match expected token env.Token!")
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
	var searchedusers slack.UserData       // slice of slack.Users to store returned users
	var searchedgroups slack.UserGroupData // slice of slack.UserGroups to store returned usergroups
	// establish slack connection
	sl := slack.New(env.UserToken, ctx)
	// get the user list
	users, err := sl.UsersList()
	if err != nil {
		log.Errorf(ctx, "ERROR error retrieving userlist, err=%s", err)
	}
	// search the user & groups list for the user-specified string
	// go through the userlist first
	for _, user := range users {
		// discard bots/non-real users by looking for Slack users that have an email address in their profile (bots don't have this)
		if strings.Contains(user.Profile.Email, "@") && user.Deleted != true {
			// search username, first & last name fields for the user-specified string
			if ciContains(user.Name, name) || ciContains(user.Profile.RealName, name) || ciContains(user.Profile.FirstName, name) || ciContains(user.Profile.LastName, name) {
				// we have a hit! add this user to the slice of slack.Users
				searchedusers = append(searchedusers, user)
				if env.Debug {
					log.Debugf(ctx, "DEBUG   user: Id=%s, Name=%s, RealName=%s, Phone=%s", user.Id, user.Name, user.Profile.RealName, user.Profile.Phone)
				}
			}
		}
	}
	// get the usergroup list
	usergroups, err := sl.UserGroupsList()
	if err != nil {
		log.Errorf(ctx, "error retrieving usergrouplist, err=%s", err)
	}
	// go through the grouplist second
	for _, group := range usergroups {
		// search group name, description & group handle fields for the user-specified string
		if ciContains(group.Handle, name) || ciContains(group.Description, name) || ciContains(group.Name, name) {
			// we have a hit! add this group to the slice of slack.UserGroups
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
		Link_names: 1,
		// ResponseType: "in_channel",
		Text: textpayload,
	}
	// response = response + fmt.Sprintf("DEBUG request PostForm value %s=%s\n", k, v)
	js, _ := json.Marshal(payload)
	// send it
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
	env.Debugstring = os.Getenv("DEBUGSTRING")
	env.SrcHome = os.Getenv("SRC_HOME")
	env.Team = os.Getenv("SLACK_TEAM")
	env.Token = os.Getenv("SLACK_TOKEN")
	env.UserToken = os.Getenv("SLACK_USER_TOKEN")
	env.Version = os.Getenv("VERSION")
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
	// check to see is debug mode is being enabled remotely
	if ciContains(hook.Text, env.Debugstring) {
		// set debug mode to tue
		env.Debug = true
		// remove "debug" from the user's search string
		ss := strings.Replace(hook.Text, " "+env.Debugstring, "", -1)
		hook.Text = ss
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
			response = response + fmt.Sprintf("*Users matching \"%s\":*\n", hook.Text)
			// sort the userlist and lets go through it
			sort.Sort(userlist)
			for _, user := range userlist {
				// if a user doesn't have a phone number, UNLIST them
				if user.Profile.Phone == "" {
					phone = "no phone # entered"
				} else {
					phone = user.Profile.Phone
				}
				// build up the user data in our response 1 line at a time
				response = response + fmt.Sprintf("%s %s: :dir_phone: %s :dir_email: <mailto:%s|%s> :slack: <@%s|%s>\n", user.Profile.FirstName, user.Profile.LastName, phone, user.Profile.Email, user.Profile.Email, user.Id, user.Name)
			}
		}
		// now go through the usergroup list
		if len(usergrouplist) > 0 {
			response = response + fmt.Sprintf("*Groups matching \"%s\":*\n", hook.Text)
			// sort the grouplist and lets go through it
			sort.Sort(usergrouplist)
			for _, usergroup := range usergrouplist {
				// build up the group data in our response 1 line at a time
				response = response + fmt.Sprintf("%s :slack: <!subteam^%s>\n", usergroup.Name, usergroup.ID)
			}
		}
	} else {
		// if there's less than 1 user or usergroup in the list, there's nothing to show
		response = fmt.Sprintf("I'm sorry, but I was not able to find a user or group using your search term \"%s\"! :confused:", hook.Text)
	}
	// debug heaven! dump basically everything. and why not indeed!
	if env.Debug {
		response = response + fmt.Sprintf("*Debug Data:*\n")
		response = response + fmt.Sprintf("DEBUG env.Debug=%v, env.Team=%s, env.version=%s, env.SrcHome=%s\n", env.Debug, env.Team, env.Version, env.SrcHome)
		// response from Slack
		response = response + fmt.Sprintf("DEBUG response from Slack Method=%s, Host=%s, URL=%s, Proto=%s, RemoteAddr=%s, Content-Length=%d\n", r.Method, r.Host, r.URL, r.Proto, r.RemoteAddr, r.ContentLength)
		// response from Slack headers
		for k, v := range r.Header {
			response = response + fmt.Sprintf("DEBUG response from Slack Header %s=%s\n", k, v)
		}
		// response from Slack formdata
		for k, v := range r.PostForm {
			// exclude sensitive things from debug output
			if strings.Contains(k, "response_url") || strings.Contains(k, "token") {
				response = response + fmt.Sprintf("DEBUG response from Slack PostForm %s=REDACTED\n", k)
			} else {
				response = response + fmt.Sprintf("DEBUG response from Slack PostForm %s=%s\n", k, v)
			}
		}
		response = response + fmt.Sprintf("DEBUG response from Slack len(userlist)=%d, len(usergrouplist)=%d\n", len(userlist), len(usergrouplist))
		response = response + fmt.Sprintf("DEBUG hook.TeamID=%s, hook.TeamDomain=%s, hook.UserName=%s, hook.UserID=%s, hook.ChannelName=%s, hook.ChannelID=%s, hook.Text=%s\n", hook.TeamID, hook.TeamDomain, hook.UserName, hook.UserID, hook.ChannelName, hook.ChannelID, hook.Text)
	}
	sendit(ctx, w, response)
	// reset debug mode
	env.Debug = false
	return
}

// EOF
