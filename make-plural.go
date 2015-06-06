package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/template"
    "flag"
)

func get(url, key string, headers *string) (map[string]map[string]string, error) {
	fmt.Print("GET ", url)

	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if 200 != response.StatusCode {
		return nil, fmt.Errorf(response.Status)
	}

	contents, err := ioutil.ReadAll(response.Body)

	var document map[string]map[string]json.RawMessage
	err = json.Unmarshal([]byte(contents), &document)
	if nil != err {
		return nil, err
	}

	if _, ok := document["supplemental"]; !ok {
		return nil, fmt.Errorf("Data does not appear to be CLDR data")
	}
	*headers += fmt.Sprintf("//\n// URL: %s\n", url)

	{
		var version map[string]string
		err = json.Unmarshal(document["supplemental"]["version"], &version)
		if nil != err {
			return nil, err
		}
		*headers += fmt.Sprintf("// %s\n", version["_number"])
	}

	{
		var generation map[string]string
		err = json.Unmarshal(document["supplemental"]["generation"], &generation)
		if nil != err {
			return nil, err
		}
		*headers += fmt.Sprintf("// %s\n", generation["_date"])
	}

	var data map[string]map[string]string
	err = json.Unmarshal(document["supplemental"]["plurals-type-"+key], &data)
	if nil != err {
		return nil, err
	}
	return data, nil
}

func part2code(left, operator, right string) string {
	var out []string

	conditions := strings.Split(right, ",")
	for _, condition := range conditions {
		pos := strings.Index(condition, "..")

		if -1 != pos {
			if "!=" == operator {
				out = append(out, fmt.Sprintf("%s < %s || %s > %s", left, condition[:pos], left, condition[pos+2:]))
			} else {
				out = append(out, fmt.Sprintf("%s >= %s && %s <= %s", left, condition[:pos], left, condition[pos+2:]))
			}
		} else {
			out = append(out, fmt.Sprintf("%s %s %s", left, operator, condition))
		}
	}

	if 1 == len(out) {
		return out[0]
	} else if "!=" == operator {
		return "(" + strings.Join(out, ") && (") + ")"
	}
	return "(" + strings.Join(out, ") || (") + ")"
}

func pattern2code(input string, ptr_keys *[]string) []string {
	left, short, operator, logic := "", "", "", ""

	var stmt []string
	var conditions [][]string

	buf := ""
loop:
	for _, char := range input {
		switch char {
		default:
			buf += string(char)

		case '@':
			break loop

		case ' ':

		case '=':
			if "" != buf {
				left, operator, buf = buf, "==", ""
				short = toVar(left, ptr_keys)
			}

		case '!':
			left, operator, buf = buf, "!=", ""
			short = toVar(left, ptr_keys)
		}

		if "" != buf {
			pos := strings.Index(buf, "and")

			if -1 != pos {
				if "OR" == logic {
					conditions = append(conditions, stmt)
					stmt = []string{}
				}
				stmt = append(stmt, part2code(short, operator, buf[:pos]))

				buf, left, operator, logic = "", "", "", "AND"
			} else {
				pos = strings.Index(buf, "or")

				if -1 != pos {
					if "OR" == logic {
						conditions = append(conditions, stmt)
						stmt = []string{}
					}
					stmt = append(stmt, part2code(short, operator, buf[:pos]))

					buf, left, operator, logic = "", "", "", "OR"
				}
			}
		}
	}

	if "" != buf {
		if "OR" == logic {
			if len(stmt) > 0 {
				conditions = append(conditions, stmt, []string{part2code(short, operator, buf)})
			} else {
				conditions = append(conditions, []string{part2code(short, operator, buf)})
			}
		} else {
			stmt = append(stmt, part2code(short, operator, buf))
			conditions = append(conditions, stmt)
		}
	}

	var result []string
	for _, condition := range conditions {
		if len(condition) > 1 {
			result = append(result, "("+strings.Join(condition, ") && (")+")")
		} else {
			result = append(result, condition[0])
		}
	}
	return result
}

func rule2code(key string, data map[string]string, ptr_keys *[]string, padding string) string {
	if input, ok := data["pluralRule-count-"+key]; ok {
		result := ""

		if "other" == key {
			result += padding + "default:\n"
		} else {
			cases := pattern2code(input, ptr_keys)
			result += "\n" + padding + "case " + strings.Join(cases, ", ") + ":\n"
		}
		result += padding + "\treturn \"" + key + "\"\n"
		return result
	}
	return ""
}

func map2code(data map[string]string, ptr_keys *[]string, padding string) string {
	if len(data) > 1 {
		result := padding + "switch {\n"
		result += rule2code("other", data, ptr_keys, padding)
		result += rule2code("zero", data, ptr_keys, padding)
		result += rule2code("one", data, ptr_keys, padding)
		result += rule2code("two", data, ptr_keys, padding)
		result += rule2code("few", data, ptr_keys, padding)
		result += rule2code("many", data, ptr_keys, padding)
		result += padding + "}\n"
		return result
	}
	return padding + "return \"other\"\n"
}

func culture2code(ordinals, plurals map[string]string, padding string) (string, string) {
	var code string
	var keys []string

	if nil == ordinals {
		code = map2code(plurals, &keys, padding)
	} else {
		code = padding + "if ordinal {\n"
		code += map2code(ordinals, &keys, padding+"\t")
		code += padding + "}\n\n"
		code += map2code(plurals, &keys, padding)
	}

	vars := ""
	max := len(keys)

	if max > 0 {
		// tokens :
		// f : decimal part
		// i : int part
		// v : len(f)
		// t : f.replace(/0+$/, '')
		var_f := varname('f', keys)
		var_i := varname('i', keys)
		var_v := varname('v', keys)
		var_t := varname('t', keys)

		if "_" != var_i && "_" == var_f && "_" == var_v && "_" == var_t {
			vars += padding + "i := int(n)\n"

			nMod(&keys)
		} else {
			if "_" != var_f || "_" != var_i || "_" != var_v || "_" != var_t {
				vars += padding + fmt.Sprintf("%s, %s, %s, %s := fivt(n)\n", var_f, var_i, var_v, var_t)
			}

			if nMod(&keys) {
				if "_" == var_i {
					vars += padding + "i := int(n)\n"
				}
			}
		}

		for i := 0; i < max; i += 2 {
			k := keys[i]
			v := keys[i+1]

			if k != v {
				vars += padding + k + " := " + v + "\n"
			}
		}

		vars += "\n"
	}
	return vars, code
}

func toVar(input string, ptr_keys *[]string) string {
	if "n" == input {
		return "n"
	}

	var short string

	if pos := strings.Index(input, "%"); -1 != pos {
		short = input[:pos] + input[pos+1:]
		input = input[:pos] + " % " + input[pos+1:]
	} else {
		short = input
	}

	exists := false
	for i := 0; i < len(*ptr_keys); i += 2 {
		if (*ptr_keys)[i] == short {
			exists = true
			break
		}
	}

	if !exists {
		*ptr_keys = append(*ptr_keys, short, input)
	}
	return short
}

func nMod(ptr_keys *[]string) bool {
	result := false
	keys := *ptr_keys
	for i := 0; i < len(keys); i += 2 {
		pos := strings.Index(keys[i+1], "n %")
		if -1 != pos {
			result = true
			(*ptr_keys)[i+1] = "i %" + keys[i+1][pos+3:]
		}
	}
	return result
}

func varname(char uint8, keys []string) string {
	for i := 0; i < len(keys); i += 2 {
		if char == keys[i][0] {
			return string(char)
		}
	}
	return "_"
}

type Item struct {
	Culture, Vars, Impl string
}

func createSource(headers string, ptr_plurals, ptr_ordinals *map[string]map[string]string) error {
	var cultures []string
    if "*" == *user_culture {
        // On sait que len(ordinals) <= len(plurals)
        for culture, _ := range *ptr_plurals {
            cultures = append(cultures, culture)
        }
    } else {
        for _, culture := range strings.Split(*user_culture, ",") {
            culture = strings.TrimSpace(culture)

            if _, ok := (*ptr_plurals)[culture]; !ok {
                return fmt.Errorf("Aborted, `%s` not found...", culture)
            }
            cultures = append(cultures, culture)
        }
    }
	sort.Strings(cultures)

	if 0 == len(cultures) {
		return fmt.Errorf("Not enough data to create source...")
	}

	tmpl, err := template.ParseFiles("plural.tmpl")
	if nil != err {
		return err
	}

	file, err := os.Create("plural/func.go")
	if nil != err {
		return err
	}
	defer file.Close()

	var items []Item
	for _, culture := range cultures {
		fmt.Print(culture)

		plurals := (*ptr_plurals)[culture]

		if nil == plurals {
			fmt.Println(" \u2717 - Plural not defined")
		} else if _, ok := plurals["pluralRule-count-other"]; !ok {
			fmt.Println(" \u2717 - Plural missing mandatory `other` choice...")
		} else {
			ordinals := (*ptr_ordinals)[culture]
			if nil != ordinals {
				if _, ok := ordinals["pluralRule-count-other"]; !ok {
					fmt.Println(" \u2717 - Ordinal missing the mandatory `other` choice...")
					continue
				}
			}

			vars, code := culture2code(ordinals, plurals, "\t\t")
			items = append(items, Item{culture, vars, code})

			fmt.Println(" \u2713")
		}
	}
	return tmpl.Execute(file, struct {
		Headers string
		Items   []Item
	}{headers, items})
}

var user_culture = flag.String("culture", "*", "Culture subset")

func main() {
    flag.Parse()

	var headers string

	ordinals, err := get("https://github.com/unicode-cldr/cldr-core/raw/master/supplemental/ordinals.json", "ordinal", &headers)
	if nil != err {
		fmt.Println(" \u2717")
		fmt.Println(err)
	} else {
		fmt.Println(" \u2713")

		plurals, err := get("https://github.com/unicode-cldr/cldr-core/raw/master/supplemental/plurals.json", "cardinal", &headers)
		if nil != err {
			fmt.Println(" \u2717")
			fmt.Println(err)
		} else {
			fmt.Println(" \u2713")

			err = createSource(headers, &plurals, &ordinals)
			if nil != err {
				fmt.Println(err, "(╯°□°）╯︵ ┻━┻")
			} else {
				fmt.Println("Succeed (ッ)")
			}
		}
	}
}
