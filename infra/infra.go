package infra

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/dop251/goja"
	"github.com/google/uuid"
	"github.com/sagiforbes/banai/utils/fsutils"
	"github.com/sirupsen/logrus"
)

//ErrSecretNotFound return when the secret was not found in secret manager
var ErrSecretNotFound = errors.New("Secret not found")

//Banai banai main struct
type Banai struct {
	Jse          *goja.Runtime
	TmpDir       string
	Logger       *logrus.Logger
	stashFolder  string
	secretFolder string

	secrets map[string]secretStruct
}

//NewBanai create new banai struct object
func NewBanai() *Banai {
	ret := &Banai{
		Jse:     goja.New(),
		Logger:  logrus.New(),
		secrets: make(map[string]secretStruct),
	}
	ret.Jse.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))
	ret.TmpDir, _ = filepath.Abs("./.banai")
	ret.stashFolder = filepath.Join(ret.TmpDir, "stash")
	ret.secretFolder = filepath.Join(ret.TmpDir, "sec")
	os.RemoveAll(ret.stashFolder)
	os.MkdirAll(ret.stashFolder, 0700)
	os.RemoveAll(ret.secretFolder)
	os.MkdirAll(ret.secretFolder, 0700)

	return ret
}

//PanicOnError return Value typed panic so javascript will get exception
func (b Banai) PanicOnError(e error, t ...string) {
	if e != nil {
		var msg string
		if t != nil {
			msg = fmt.Sprintf("%s %s", t, e)
			b.Logger.Error(msg)
			panic(b.Jse.ToValue(msg))

		} else {
			msg = fmt.Sprint(e)
			b.Logger.Error(msg)
			panic(b.Jse.ToValue(msg))
		}

	}
}

//Close should be call at the end of using banai to remove all allocated resource during banai execution
func (b Banai) Close() {
	os.RemoveAll(b.TmpDir)

}

//*********************************************************************************

//Save stashs file CONTENT
func (b Banai) Save(fileName string) (string, error) {
	abs, e := filepath.Abs(fileName)
	if e != nil {
		return "", e
	}
	stashID := uuid.NewString()

	e = fsutils.CopyfsItem(abs, stashID)
	if e != nil {
		return "", e
	}
	return stashID, nil
}

//Load restore the CONTENT of a previously stashed file
func (b Banai) Load(stashID string) ([]byte, error) {
	path := filepath.Join(b.stashFolder, stashID)
	_, e := os.Stat(path)
	if e != nil {
		return nil, e
	}

	f, e := ioutil.ReadFile(path)
	if e != nil {
		return nil, e
	}

	return f, nil

}

//*********************************************************************************

//AddStringSecret add secret string
func (b Banai) AddStringSecret(secretID string, value string) {
	b.secrets[secretID] = secretText{
		Text: value,
	}

}

//AddSSHWithPrivate add secret string
func (b Banai) AddSSHWithPrivate(secretID string, user string, privateKey string, passphrase string) {
	b.secrets[secretID] = secretSSHWithPrivate{
		User:       user,
		PrivateKey: privateKey,
		Passphrase: passphrase,
	}

}

//AddUserPassword secret of type user name password
func (b Banai) AddUserPassword(secretID, user, password string) {
	b.secrets[secretID] = secretUserPassword{
		User:     user,
		Password: password,
	}
}

//*********************************************************************************

//SecretInfo Base interface of returned secrets
type SecretInfo interface {
	GetType() string
}

//TextSecret return string secret
type TextSecret struct {
	Text string `json:"text,omitempty"`
}

//GetType type of secret
func (t TextSecret) GetType() string {
	return "text"
}

//SSHWithPrivate info to use when using ssh with private key
type SSHWithPrivate struct {
	User           string `json:"user,omitempty"`
	PrivatekeyFile string `json:"privateKeyFile,omitempty"`
	Passfrase      string `json:"passfrase,omitempty"`
}

//GetType get secret info type
func (t SSHWithPrivate) GetType() string {
	return "ssh"
}

//UserPassword info to use when using user password
type UserPassword struct {
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
}

//GetType get secret info type
func (t UserPassword) GetType() string {
	return "userpass"
}

//GetSecret add secret string
func (b Banai) GetSecret(secretID string) (SecretInfo, error) {
	v, ok := b.secrets[secretID]
	if !ok {
		return nil, ErrSecretNotFound
	}

	var ret SecretInfo

	switch v.GetType() {
	case "text":
		s := TextSecret{
			Text: v.(secretText).Text,
		}
		ret = s
	case "ssh":
		i := v.(secretSSHWithPrivate)
		fn := filepath.Join(b.secretFolder, secretID)
		err := ioutil.WriteFile(fn, []byte(i.PrivateKey), 600)
		if err != nil {
			return nil, ErrSecretNotFound
		}
		s := SSHWithPrivate{
			User:           v.(secretSSHWithPrivate).User,
			PrivatekeyFile: fn,
			Passfrase:      i.Passphrase,
		}
		ret = s
	case "userpass":
		i := v.(secretUserPassword)
		s := UserPassword{
			User:     i.User,
			Password: i.Password,
		}
		ret = s
	}

	return ret, nil

}

//*********************************************************************************
