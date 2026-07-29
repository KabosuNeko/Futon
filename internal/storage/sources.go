package storage

func LoadSources() ([]string, error) {
	ud, err := LoadUserData()
	if err != nil {
		return nil, err
	}
	if ud == nil {
		return nil, nil
	}
	return ud.Sources, nil
}

func SaveSources(names []string) error {
	ud, err := LoadUserData()
	if err != nil {
		return err
	}
	if ud == nil {
		ud = &UserData{Sources: names}
	} else {
		ud.Sources = names
	}
	return SaveUserData(ud)
}
