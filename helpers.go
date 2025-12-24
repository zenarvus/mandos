package main
import ("fmt"; "log"; "os"; "path"; "path/filepath"; "regexp"; "strings";)
// Regular Expressions
var linkRe = regexp.MustCompile(`\]\(/([^)?#]*)[^)]*\)|<[^>]+src="/([^"?#]+)[^>]`) // Extract internal markdown links or internal html links inside src. Do not capture after ? or #

var notesPath = getNotesPath() //it does not and should not have a slash suffix.
var onlyPublic = getEnvValue("ONLY_PUBLIC")
var indexPage = getEnvValue("INDEX")

func isServed(publicField bool)bool{return onlyPublic=="no" || publicField==true}

func getEnvValue(key string)string{
	// If environment variable has a value, return it.
	if os.Getenv(key) != "" {return os.Getenv(key)}
	// If no value is assigned to the environment variable, use the default one or give an error.
	switch key {
	case "MD_FOLDER": log.Fatal(fmt.Errorf("Please specify markdown folder path with MD_FOLDER environment variable."))
	case "INDEX": return "index.md"
	case "PORT": return "9700"
	case "ONLY_PUBLIC": return "yes"
	case "CONTENT_SEARCH": return "false"
	//The location of the templates. Relative to the MD_FOLDER. Default is mandos.
	case "MD_TEMPLATES": return path.Join(getNotesPath(), "mandos")
	}
	return ""
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
