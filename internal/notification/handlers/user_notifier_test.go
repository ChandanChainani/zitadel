package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/zitadel/zitadel/internal/notification/messages"

	"github.com/golang/mock/gomock"
	statik_fs "github.com/rakyll/statik/fs"
	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"

	"github.com/zitadel/zitadel/internal/crypto"
	"github.com/zitadel/zitadel/internal/eventstore"
	"github.com/zitadel/zitadel/internal/eventstore/repository"
	es_repo_mock "github.com/zitadel/zitadel/internal/eventstore/repository/mock"
	channel_mock "github.com/zitadel/zitadel/internal/notification/channels/mock"
	"github.com/zitadel/zitadel/internal/notification/channels/smtp"
	"github.com/zitadel/zitadel/internal/notification/channels/twilio"
	"github.com/zitadel/zitadel/internal/notification/channels/webhook"
	"github.com/zitadel/zitadel/internal/notification/handlers/mock"
	"github.com/zitadel/zitadel/internal/notification/senders"
	"github.com/zitadel/zitadel/internal/notification/types"
	"github.com/zitadel/zitadel/internal/query"
	"github.com/zitadel/zitadel/internal/repository/user"
)

const (
	logoURL               = "logo.png"
	policyID              = "policy1"
	userID                = "user1"
	loginName             = "loginName1"
	orgID                 = "org1"
	eventOrigin           = "https://triggered.here"
	assetsPath            = "/assets/v1"
	instancePrimaryDomain = "primary.domain"
	externalDomain        = "external.domain"
	externalPort          = 3000
	externalSecure        = false
	externalProtocol      = "http"
	lastEmail             = "last@email.com"
)

func Test_userNotifier_reduceInitCodeAdded(t *testing.T) {
	expectMailSubject := "Initialize User"
	tests := []struct {
		name string
		test func(*gomock.Controller, *mock.MockQueries, *mock.MockCommands) (fields, args, want)
	}{{
		name: "asset url with event trigger url",
		test: func(ctrl *gomock.Controller, queries *mock.MockQueries, commands *mock.MockCommands) (f fields, a args, w want) {
			givenTemplate := "{{.LogoURL}}"
			expectContent := fmt.Sprintf("%s%s/%s/%s", eventOrigin, assetsPath, policyID, logoURL)
			w.message = messages.Email{
				Recipients: []string{lastEmail},
				Subject:    expectMailSubject,
				Content:    expectContent,
			}
			codeAlg, code := cryptoValue(t, ctrl, "testcode")
			smtpAlg, _ := cryptoValue(t, ctrl, "smtppw")
			queries.EXPECT().NotificationProviderByIDAndType(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(&query.DebugNotificationProvider{}, nil)
			expectTemplateQueries(queries, givenTemplate)
			commands.EXPECT().HumanInitCodeSent(gomock.Any(), orgID, userID).Return(nil)
			return fields{
					queries:  queries,
					commands: commands,
					es: eventstore.NewEventstore(eventstore.TestConfig(
						es_repo_mock.NewRepo(t).ExpectFilterEvents(),
					)),
					userDataCrypto:     codeAlg,
					SMTPPasswordCrypto: smtpAlg,
					SMSTokenCrypto:     nil,
				}, args{
					event: &user.HumanInitialCodeAddedEvent{
						BaseEvent: *eventstore.BaseEventFromRepo(&repository.Event{
							AggregateID:   userID,
							ResourceOwner: sql.NullString{String: orgID},
							CreationDate:  time.Now().UTC(),
						}),
						Code:              code,
						Expiry:            time.Hour,
						TriggeredAtOrigin: eventOrigin,
					},
				}, w
		},
	}, {
		name: "asset url without event trigger url",
		test: func(ctrl *gomock.Controller, queries *mock.MockQueries, commands *mock.MockCommands) (f fields, a args, w want) {
			givenTemplate := "{{.LogoURL}}"
			expectContent := fmt.Sprintf("%s://%s:%d%s/%s/%s", externalProtocol, instancePrimaryDomain, externalPort, assetsPath, policyID, logoURL)
			w.message = messages.Email{
				Recipients: []string{lastEmail},
				Subject:    expectMailSubject,
				Content:    expectContent,
			}
			codeAlg, code := cryptoValue(t, ctrl, "testcode")
			smtpAlg, _ := cryptoValue(t, ctrl, "smtppw")
			queries.EXPECT().NotificationProviderByIDAndType(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(&query.DebugNotificationProvider{}, nil)
			queries.EXPECT().SearchInstanceDomains(gomock.Any(), gomock.Any()).Return(&query.InstanceDomains{
				Domains: []*query.InstanceDomain{{
					Domain:    instancePrimaryDomain,
					IsPrimary: true,
				}},
			}, nil)
			expectTemplateQueries(queries, givenTemplate)
			commands.EXPECT().HumanInitCodeSent(gomock.Any(), orgID, userID).Return(nil)
			return fields{
					queries:  queries,
					commands: commands,
					es: eventstore.NewEventstore(eventstore.TestConfig(
						es_repo_mock.NewRepo(t).ExpectFilterEvents(),
					)),
					userDataCrypto:     codeAlg,
					SMTPPasswordCrypto: smtpAlg,
				}, args{
					event: &user.HumanInitialCodeAddedEvent{
						BaseEvent: *eventstore.BaseEventFromRepo(&repository.Event{
							AggregateID:   userID,
							ResourceOwner: sql.NullString{String: orgID},
							CreationDate:  time.Now().UTC(),
						}),
						Code:   code,
						Expiry: time.Hour,
					},
				}, w
		},
	}, {
		name: "button url with event trigger url",
		test: func(ctrl *gomock.Controller, queries *mock.MockQueries, commands *mock.MockCommands) (f fields, a args, w want) {
			givenTemplate := "{{.URL}}"
			testCode := "testcode"
			expectContent := fmt.Sprintf("%s/ui/login/user/init?userID=%s&loginname=%s&code=%s&orgID=%s&passwordset=%t", eventOrigin, userID, loginName, testCode, orgID, false)
			w.message = messages.Email{
				Recipients: []string{lastEmail},
				Subject:    expectMailSubject,
				Content:    expectContent,
			}
			codeAlg, code := cryptoValue(t, ctrl, testCode)
			smtpAlg, _ := cryptoValue(t, ctrl, "smtppw")
			queries.EXPECT().NotificationProviderByIDAndType(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(&query.DebugNotificationProvider{}, nil)
			expectTemplateQueries(queries, givenTemplate)
			commands.EXPECT().HumanInitCodeSent(gomock.Any(), orgID, userID).Return(nil)
			return fields{
					queries:  queries,
					commands: commands,
					es: eventstore.NewEventstore(eventstore.TestConfig(
						es_repo_mock.NewRepo(t).ExpectFilterEvents(),
					)),
					userDataCrypto:     codeAlg,
					SMTPPasswordCrypto: smtpAlg,
					SMSTokenCrypto:     nil,
				}, args{
					event: &user.HumanInitialCodeAddedEvent{
						BaseEvent: *eventstore.BaseEventFromRepo(&repository.Event{
							AggregateID:   userID,
							ResourceOwner: sql.NullString{String: orgID},
							CreationDate:  time.Now().UTC(),
						}),
						Code:              code,
						Expiry:            time.Hour,
						TriggeredAtOrigin: eventOrigin,
					},
				}, w
		},
	}, {
		name: "button url without event trigger url",
		test: func(ctrl *gomock.Controller, queries *mock.MockQueries, commands *mock.MockCommands) (f fields, a args, w want) {
			givenTemplate := "{{.URL}}"
			testCode := "testcode"
			expectContent := fmt.Sprintf("%s://%s:%d/ui/login/user/init?userID=%s&loginname=%s&code=%s&orgID=%s&passwordset=%t", externalProtocol, instancePrimaryDomain, externalPort, userID, loginName, testCode, orgID, false)
			w.message = messages.Email{
				Recipients: []string{lastEmail},
				Subject:    expectMailSubject,
				Content:    expectContent,
			}
			codeAlg, code := cryptoValue(t, ctrl, testCode)
			smtpAlg, _ := cryptoValue(t, ctrl, "smtppw")
			queries.EXPECT().NotificationProviderByIDAndType(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(&query.DebugNotificationProvider{}, nil)
			queries.EXPECT().SearchInstanceDomains(gomock.Any(), gomock.Any()).Return(&query.InstanceDomains{
				Domains: []*query.InstanceDomain{{
					Domain:    instancePrimaryDomain,
					IsPrimary: true,
				}},
			}, nil)
			expectTemplateQueries(queries, givenTemplate)
			commands.EXPECT().HumanInitCodeSent(gomock.Any(), orgID, userID).Return(nil)
			return fields{
					queries:  queries,
					commands: commands,
					es: eventstore.NewEventstore(eventstore.TestConfig(
						es_repo_mock.NewRepo(t).ExpectFilterEvents(),
					)),
					userDataCrypto:     codeAlg,
					SMTPPasswordCrypto: smtpAlg,
					SMSTokenCrypto:     nil,
				}, args{
					event: &user.HumanInitialCodeAddedEvent{
						BaseEvent: *eventstore.BaseEventFromRepo(&repository.Event{
							AggregateID:   userID,
							ResourceOwner: sql.NullString{String: orgID},
							CreationDate:  time.Now().UTC(),
						}),
						Code:   code,
						Expiry: time.Hour,
					},
				}, w
		},
	}}
	fs, err := statik_fs.NewWithNamespace("notification")
	assert.NoError(t, err)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			queries := mock.NewMockQueries(ctrl)
			commands := mock.NewMockCommands(ctrl)
			f, a, w := tt.test(ctrl, queries, commands)
			_, err = newUserNotifier(ctrl, fs, f, a, w).reduceInitCodeAdded(a.event)
			if w.err != nil {
				w.err(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_userNotifier_reduceEmailCodeAdded(t *testing.T) {
	expectMailSubject := "Verify email"
	tests := []struct {
		name string
		test func(*gomock.Controller, *mock.MockQueries, *mock.MockCommands) (fields, args, want)
	}{{
		name: "asset url with event trigger url",
		test: func(ctrl *gomock.Controller, queries *mock.MockQueries, commands *mock.MockCommands) (f fields, a args, w want) {
			givenTemplate := "{{.LogoURL}}"
			expectContent := fmt.Sprintf("%s%s/%s/%s", eventOrigin, assetsPath, policyID, logoURL)
			w.message = messages.Email{
				Recipients: []string{lastEmail},
				Subject:    expectMailSubject,
				Content:    expectContent,
			}
			codeAlg, code := cryptoValue(t, ctrl, "testcode")
			smtpAlg, _ := cryptoValue(t, ctrl, "smtppw")
			queries.EXPECT().NotificationProviderByIDAndType(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(&query.DebugNotificationProvider{}, nil)
			expectTemplateQueries(queries, givenTemplate)
			commands.EXPECT().HumanEmailVerificationCodeSent(gomock.Any(), orgID, userID).Return(nil)
			return fields{
					queries:  queries,
					commands: commands,
					es: eventstore.NewEventstore(eventstore.TestConfig(
						es_repo_mock.NewRepo(t).ExpectFilterEvents(),
					)),
					userDataCrypto:     codeAlg,
					SMTPPasswordCrypto: smtpAlg,
					SMSTokenCrypto:     nil,
				}, args{
					event: &user.HumanEmailCodeAddedEvent{
						BaseEvent: *eventstore.BaseEventFromRepo(&repository.Event{
							AggregateID:   userID,
							ResourceOwner: sql.NullString{String: orgID},
							CreationDate:  time.Now().UTC(),
						}),
						Code:              code,
						Expiry:            time.Hour,
						URLTemplate:       "",
						CodeReturned:      false,
						TriggeredAtOrigin: eventOrigin,
					},
				}, w
		},
	}, {
		name: "asset url without event trigger url",
		test: func(ctrl *gomock.Controller, queries *mock.MockQueries, commands *mock.MockCommands) (f fields, a args, w want) {
			givenTemplate := "{{.LogoURL}}"
			expectContent := fmt.Sprintf("%s://%s:%d%s/%s/%s", externalProtocol, instancePrimaryDomain, externalPort, assetsPath, policyID, logoURL)
			w.message = messages.Email{
				Recipients: []string{lastEmail},
				Subject:    expectMailSubject,
				Content:    expectContent,
			}
			codeAlg, code := cryptoValue(t, ctrl, "testcode")
			smtpAlg, _ := cryptoValue(t, ctrl, "smtppw")
			queries.EXPECT().NotificationProviderByIDAndType(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(&query.DebugNotificationProvider{}, nil)
			queries.EXPECT().SearchInstanceDomains(gomock.Any(), gomock.Any()).Return(&query.InstanceDomains{
				Domains: []*query.InstanceDomain{{
					Domain:    instancePrimaryDomain,
					IsPrimary: true,
				}},
			}, nil)
			expectTemplateQueries(queries, givenTemplate)
			commands.EXPECT().HumanEmailVerificationCodeSent(gomock.Any(), orgID, userID).Return(nil)
			return fields{
					queries:  queries,
					commands: commands,
					es: eventstore.NewEventstore(eventstore.TestConfig(
						es_repo_mock.NewRepo(t).ExpectFilterEvents(),
					)),
					userDataCrypto:     codeAlg,
					SMTPPasswordCrypto: smtpAlg,
				}, args{
					event: &user.HumanEmailCodeAddedEvent{
						BaseEvent: *eventstore.BaseEventFromRepo(&repository.Event{
							AggregateID:   userID,
							ResourceOwner: sql.NullString{String: orgID},
							CreationDate:  time.Now().UTC(),
						}),
						Code:         code,
						Expiry:       time.Hour,
						URLTemplate:  "",
						CodeReturned: false,
					},
				}, w
		},
	}, {
		name: "button url with event trigger url",
		test: func(ctrl *gomock.Controller, queries *mock.MockQueries, commands *mock.MockCommands) (f fields, a args, w want) {
			givenTemplate := "{{.URL}}"
			testCode := "testcode"
			expectContent := fmt.Sprintf("%s/ui/login/mail/verification?userID=%s&code=%s&orgID=%s", eventOrigin, userID, testCode, orgID)
			w.message = messages.Email{
				Recipients: []string{lastEmail},
				Subject:    expectMailSubject,
				Content:    expectContent,
			}
			codeAlg, code := cryptoValue(t, ctrl, testCode)
			smtpAlg, _ := cryptoValue(t, ctrl, "smtppw")
			queries.EXPECT().NotificationProviderByIDAndType(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(&query.DebugNotificationProvider{}, nil)
			expectTemplateQueries(queries, givenTemplate)
			commands.EXPECT().HumanEmailVerificationCodeSent(gomock.Any(), orgID, userID).Return(nil)
			return fields{
					queries:  queries,
					commands: commands,
					es: eventstore.NewEventstore(eventstore.TestConfig(
						es_repo_mock.NewRepo(t).ExpectFilterEvents(),
					)),
					userDataCrypto:     codeAlg,
					SMTPPasswordCrypto: smtpAlg,
					SMSTokenCrypto:     nil,
				}, args{
					event: &user.HumanEmailCodeAddedEvent{
						BaseEvent: *eventstore.BaseEventFromRepo(&repository.Event{
							AggregateID:   userID,
							ResourceOwner: sql.NullString{String: orgID},
							CreationDate:  time.Now().UTC(),
						}),
						Code:              code,
						Expiry:            time.Hour,
						URLTemplate:       "",
						CodeReturned:      false,
						TriggeredAtOrigin: eventOrigin,
					},
				}, w
		},
	}, {
		name: "button url without event trigger url",
		test: func(ctrl *gomock.Controller, queries *mock.MockQueries, commands *mock.MockCommands) (f fields, a args, w want) {
			givenTemplate := "{{.URL}}"
			testCode := "testcode"
			expectContent := fmt.Sprintf("%s://%s:%d/ui/login/mail/verification?userID=%s&code=%s&orgID=%s", externalProtocol, instancePrimaryDomain, externalPort, userID, testCode, orgID)
			w.message = messages.Email{
				Recipients: []string{lastEmail},
				Subject:    expectMailSubject,
				Content:    expectContent,
			}
			codeAlg, code := cryptoValue(t, ctrl, testCode)
			smtpAlg, _ := cryptoValue(t, ctrl, "smtppw")
			queries.EXPECT().NotificationProviderByIDAndType(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(&query.DebugNotificationProvider{}, nil)
			queries.EXPECT().SearchInstanceDomains(gomock.Any(), gomock.Any()).Return(&query.InstanceDomains{
				Domains: []*query.InstanceDomain{{
					Domain:    instancePrimaryDomain,
					IsPrimary: true,
				}},
			}, nil)
			expectTemplateQueries(queries, givenTemplate)
			commands.EXPECT().HumanEmailVerificationCodeSent(gomock.Any(), orgID, userID).Return(nil)
			return fields{
					queries:  queries,
					commands: commands,
					es: eventstore.NewEventstore(eventstore.TestConfig(
						es_repo_mock.NewRepo(t).ExpectFilterEvents(),
					)),
					userDataCrypto:     codeAlg,
					SMTPPasswordCrypto: smtpAlg,
					SMSTokenCrypto:     nil,
				}, args{
					event: &user.HumanEmailCodeAddedEvent{
						BaseEvent: *eventstore.BaseEventFromRepo(&repository.Event{
							AggregateID:   userID,
							ResourceOwner: sql.NullString{String: orgID},
							CreationDate:  time.Now().UTC(),
						}),
						Code:         code,
						Expiry:       time.Hour,
						URLTemplate:  "",
						CodeReturned: false,
					},
				}, w
		},
	}, {
		name: "button url with url template and event trigger url",
		test: func(ctrl *gomock.Controller, queries *mock.MockQueries, commands *mock.MockCommands) (f fields, a args, w want) {
			givenTemplate := "{{.URL}}"
			urlTemplate := "https://my.custom.url/org/{{.OrgID}}/user/{{.UserID}}/verify/{{.Code}}"
			testCode := "testcode"
			expectContent := fmt.Sprintf("https://my.custom.url/org/%s/user/%s/verify/%s", orgID, userID, testCode)
			w.message = messages.Email{
				Recipients: []string{lastEmail},
				Subject:    expectMailSubject,
				Content:    expectContent,
			}
			codeAlg, code := cryptoValue(t, ctrl, testCode)
			smtpAlg, _ := cryptoValue(t, ctrl, "smtppw")
			queries.EXPECT().NotificationProviderByIDAndType(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(&query.DebugNotificationProvider{}, nil)
			expectTemplateQueries(queries, givenTemplate)
			commands.EXPECT().HumanEmailVerificationCodeSent(gomock.Any(), orgID, userID).Return(nil)
			return fields{
					queries:  queries,
					commands: commands,
					es: eventstore.NewEventstore(eventstore.TestConfig(
						es_repo_mock.NewRepo(t).ExpectFilterEvents(),
					)),
					userDataCrypto:     codeAlg,
					SMTPPasswordCrypto: smtpAlg,
					SMSTokenCrypto:     nil,
				}, args{
					event: &user.HumanEmailCodeAddedEvent{
						BaseEvent: *eventstore.BaseEventFromRepo(&repository.Event{
							AggregateID:   userID,
							ResourceOwner: sql.NullString{String: orgID},
							CreationDate:  time.Now().UTC(),
						}),
						Code:              code,
						Expiry:            time.Hour,
						URLTemplate:       urlTemplate,
						CodeReturned:      false,
						TriggeredAtOrigin: eventOrigin,
					},
				}, w
		},
	}}
	fs, err := statik_fs.NewWithNamespace("notification")
	assert.NoError(t, err)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			queries := mock.NewMockQueries(ctrl)
			commands := mock.NewMockCommands(ctrl)
			f, a, w := tt.test(ctrl, queries, commands)
			_, err = newUserNotifier(ctrl, fs, f, a, w).reduceEmailCodeAdded(a.event)
			if w.err != nil {
				w.err(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

type fields struct {
	queries            *mock.MockQueries
	commands           *mock.MockCommands
	es                 *eventstore.Eventstore
	userDataCrypto     crypto.EncryptionAlgorithm
	SMTPPasswordCrypto crypto.EncryptionAlgorithm
	SMSTokenCrypto     crypto.EncryptionAlgorithm
}
type args struct {
	event eventstore.Event
}
type want struct {
	message messages.Email
	err     assert.ErrorAssertionFunc
}

func newUserNotifier(ctrl *gomock.Controller, fs http.FileSystem, f fields, a args, w want) *userNotifier {
	channel := channel_mock.NewMockNotificationChannel(ctrl)
	if w.err == nil {
		w.message.TriggeringEvent = a.event
		channel.EXPECT().HandleMessage(&w.message).Return(nil)
	}
	return &userNotifier{
		commands: f.commands,
		queries: NewNotificationQueries(
			f.queries,
			f.es,
			externalDomain,
			externalPort,
			externalSecure,
			"",
			f.userDataCrypto,
			f.SMTPPasswordCrypto,
			f.SMSTokenCrypto,
			fs,
		),
		channels: &channels{Chain: *senders.ChainChannels(channel)},
	}
}

var _ types.ChannelChains = (*channels)(nil)

type channels struct {
	senders.Chain
}

func (c *channels) Email(context.Context) (*senders.Chain, *smtp.Config, error) {
	return &c.Chain, nil, nil
}

func (c *channels) SMS(context.Context) (*senders.Chain, *twilio.Config, error) {
	return &c.Chain, nil, nil
}

func (c *channels) Webhook(context.Context, webhook.Config) (*senders.Chain, error) {
	return &c.Chain, nil
}

func expectTemplateQueries(queries *mock.MockQueries, template string) {
	queries.EXPECT().ActiveLabelPolicyByOrg(gomock.Any(), gomock.Any(), gomock.Any()).Return(&query.LabelPolicy{
		ID: policyID,
		Light: query.Theme{
			LogoURL: logoURL,
		},
	}, nil)
	queries.EXPECT().MailTemplateByOrg(gomock.Any(), gomock.Any(), gomock.Any()).Return(&query.MailTemplate{Template: []byte(template)}, nil)
	queries.EXPECT().GetNotifyUserByID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&query.NotifyUser{
		ID:                 userID,
		ResourceOwner:      orgID,
		LastEmail:          lastEmail,
		PreferredLoginName: loginName,
	}, nil)
	queries.EXPECT().GetDefaultLanguage(gomock.Any()).Return(language.English)
	queries.EXPECT().CustomTextListByTemplate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(2).Return(&query.CustomTexts{}, nil)
}

func cryptoValue(t *testing.T, ctrl *gomock.Controller, value string) (*crypto.MockEncryptionAlgorithm, *crypto.CryptoValue) {
	encAlg := crypto.NewMockEncryptionAlgorithm(ctrl)
	encAlg.EXPECT().Algorithm().AnyTimes().Return("enc")
	encAlg.EXPECT().EncryptionKeyID().AnyTimes().Return("id")
	encAlg.EXPECT().DecryptionKeyIDs().AnyTimes().Return([]string{"id"})
	encAlg.EXPECT().DecryptString(gomock.Any(), gomock.Any()).AnyTimes().Return(value, nil)
	encAlg.EXPECT().Encrypt(gomock.Any()).AnyTimes().Return(make([]byte, 0), nil)
	code, err := crypto.Encrypt(nil, encAlg)
	assert.NoError(t, err)
	return encAlg, code
}
