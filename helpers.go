package main
import ("fmt"; "log"; "os"; "path"; "path/filepath"; "strings"; "unicode"; "unicode/utf8"; "bytes"; "strconv"; "github.com/cespare/xxhash/v2";)

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

func SafeJoin(basePath, relPath string) (string) {
    // 1. Join and Clean the path in one go
    // filepath.Join calls filepath.Clean, which resolves ".." and "."
    finalPath := filepath.Join(basePath, relPath)

    // 2. Simple prefix check
    // We use filepath.Rel to ensure the path is truly a subpath. 
    // This is more robust than strings.HasPrefix because it handles 
    // cases like "/var/lib" vs "/var/lib-shortcut"
    rel, err := filepath.Rel(basePath, finalPath)
    if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
        return ""
    }

    return finalPath
}


// Map of common unicode runes to ASCII replacements.
var repl = map[rune]string{
	'á': "a", 'à': "a", 'â': "a", 'ä': "a", 'ã': "a", 'å': "a",
	'é': "e", 'è': "e", 'ê': "e", 'ë': "e",
	'í': "i", 'ì': "i", 'î': "i", 'ï': "i", 'ı': "i",
	'ó': "o", 'ò': "o", 'ô': "o", 'ö': "o", 'õ': "o",
	'ú': "u", 'ù': "u", 'û': "u", 'ü': "u",
	'ğ': "g", 'ñ': "n", 'ç': "c",
	'ý': "y", 'ÿ': "y",
	'þ': "th", 'ð': "d",
	'æ': "ae", 'œ': "oe",
}
// Slugify converts input bytes to a slug bytes slice using sep (e.g., '-').
// Result is lowercased ASCII; non-transliterable runes are removed or become sep.
func Slugify(in []byte, sep byte) []byte {
	if len(in) == 0 {return nil}
	var out bytes.Buffer
	out.Grow(len(in))
	prevSep := false

	for len(in) > 0 {
		r, size := utf8.DecodeRune(in)
		in = in[size:]

		// ASCII fast-path
		if r < utf8.RuneSelf {
			switch {
			case r >= 'A' && r <= 'Z':
				out.WriteByte(byte(r + ('a' - 'A'))) // to lowercase
				prevSep = false
			case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
				out.WriteByte(byte(r))
				prevSep = false
			default:
				if !prevSep && out.Len() > 0 {
					out.WriteByte(sep)
					prevSep = true
				}
			}
			continue
		} else {
			// Transliterate common runes
			if s, ok := repl[unicode.ToLower(r)]; ok {
				// write transliteration as lowercase
				out.WriteString(s)
				prevSep = false
				continue
			}
		}

		// For letters in other scripts try to use unicode.IsLetter -> drop if not ASCII
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			// Attempt a naive decomposition: remove diacritics isn't cheap; drop unknown non-ASCII.
			// To keep short and fast, we omit expensive normalization and treat as separator.
			if !prevSep && out.Len() > 0 {
				out.WriteByte(sep)
				prevSep = true
			}
			continue
		}

		// Other runes -> separator
		if !prevSep && out.Len() > 0 {
			out.WriteByte(sep)
			prevSep = true
		}
	}
	// Trim trailing separator
	b := out.Bytes()
	if len(b) > 0 && b[len(b)-1] == sep { b = b[:len(b)-1] }
	// Trim leading separator
	if len(b) > 0 && b[0] == sep { b = b[1:] }
	// Return a copy to ensure external mutation won't affect internal buffer
	res := make([]byte, len(b))
	copy(res, b)
	return res
}
