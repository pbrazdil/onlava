package helper

var secrets struct {
	HelperSecret string
}

func Value() string {
	return secrets.HelperSecret
}
