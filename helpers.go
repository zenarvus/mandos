package main
import ("fmt"; "log"; "os"; "path"; "path/filepath"; "strings"; "strconv"; "github.com/cespare/xxhash/v2";)

var notesPath = getNotesPath() //it does not and should not have a slash suffix.
var onlyPublic = getEnvValue("ONLY_PUBLIC")
var indexPage = getEnvValue("INDEX")

func isServed(publicField bool)bool{return onlyPublic=="no" || publicField==true}

var envValues = make(map[string]string)
func getEnvValue(key string)string{
	// If it's in the map, return it.
	if envValues[key] != "" {return envValues[key]}

	// If environment variable has a value, return it.
	if os.Getenv(key) != "" { envValues[key]=os.Getenv(key); return envValues[key]}

	// If no value is assigned to the environment variable, use the default one or give an error.
	switch key {
	case "MD_FOLDER":
		log.Fatal(fmt.Errorf("Please specify markdown folder path with MD_FOLDER environment variable."))
	case "INDEX":
		envValues[key]="index.md"; return envValues[key]
	case "PORT":
		envValues["PORT"]="9700"; return envValues["PORT"]
	case "ONLY_PUBLIC":
		envValues[key]="yes"; return envValues[key]
	case "CONTENT_SEARCH":
		envValues[key]="false"; return envValues[key]
	case "CACHE_FOLDER":
		userCache, err := os.UserCacheDir();
		if err!=nil{log.Fatalln("Cache dir could not be determined. Please specify it using CACHE_FOLDER", err)}
		envValues[key] = filepath.Join(userCache,"mandos")
		return envValues[key]

	//The location of the templates. Relative to the MD_FOLDER. Default is mandos.
	case "MD_TEMPLATES": envValues[key]=path.Join(getNotesPath(), "mandos"); return envValues[key]
	}
	return ""
}

// Used to convert some environment variables to integers. So it's okay to give fatal errors.
func convertToInt(str string) int {
	int, err := strconv.Atoi(str)
	if err!=nil{log.Fatalln("Environment variable error:",err)}

	return int
}

func getNotesPath() string {
	// Follow the system links and get the md-folder path.
	p, err := filepath.EvalSymlinks(getEnvValue("MD_FOLDER")); if err!=nil{log.Fatal(err)}
	// Replaces ~ with the user's home directory.
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir(); if err != nil {log.Fatal(err)}
		p = filepath.Join(home, p[2:])
	}
	// Converts a relative path to an absolute path.
	p, err = filepath.Abs(p); if err != nil {log.Fatal(err)}
	return strings.TrimSuffix(p, "/")
}

func GetQueryKey(query string, args ...any) string {
    h := xxhash.New()
    h.Write([]byte(query))
    for _, arg := range args {
        h.Write([]byte(fmt.Sprintf("%v", arg)))
    }
    return fmt.Sprintf("%016x", h.Sum64())
}
