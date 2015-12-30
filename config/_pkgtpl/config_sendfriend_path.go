// +build ignore

package sendfriend

import (
	"github.com/corestoreio/csfw/config/model"
)

// PathSendfriendEmailEnabled => Enabled.
// SourceModel: Otnegam\Config\Model\Config\Source\Yesno
var PathSendfriendEmailEnabled = model.NewBool(`sendfriend/email/enabled`)

// PathSendfriendEmailTemplate => Select Email Template.
// Email template chosen based on theme fallback when "Default" option is
// selected.
// SourceModel: Otnegam\Config\Model\Config\Source\Email\Template
var PathSendfriendEmailTemplate = model.NewStr(`sendfriend/email/template`)

// PathSendfriendEmailAllowGuest => Allow for Guests.
// SourceModel: Otnegam\Config\Model\Config\Source\Yesno
var PathSendfriendEmailAllowGuest = model.NewBool(`sendfriend/email/allow_guest`)

// PathSendfriendEmailMaxRecipients => Max Recipients.
var PathSendfriendEmailMaxRecipients = model.NewStr(`sendfriend/email/max_recipients`)

// PathSendfriendEmailMaxPerHour => Max Products Sent in 1 Hour.
var PathSendfriendEmailMaxPerHour = model.NewStr(`sendfriend/email/max_per_hour`)

// PathSendfriendEmailCheckBy => Limit Sending By.
// SourceModel: Otnegam\SendFriend\Model\Source\Checktype
var PathSendfriendEmailCheckBy = model.NewStr(`sendfriend/email/check_by`)
