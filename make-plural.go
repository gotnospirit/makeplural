package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/template"
	"time"
)

type (
	Source interface {
		Culture() string
		CultureId() string
		Code() string
	}

	Test interface {
		toString() string
	}

	FuncSource struct {
		culture, vars, impl string
	}

	UnitTestSource struct {
		culture string
		tests   []Test
	}

	UnitTest struct {
		ordinal         bool
		expected, value string
	}
)

func (x FuncSource) Culture() string {
	return x.culture
}

func (x FuncSource) CultureId() string {
	return Sanitize(x.culture)
}

func (x FuncSource) Code() string {
	result := ""
	if "" != x.vars {
		result += x.vars + "\n"
	}
	result += x.impl
	return result
}

func (x UnitTestSource) Culture() string {
	return x.culture
}

func (x UnitTestSource) CultureId() string {
	return Sanitize(x.culture)
}

func (x UnitTestSource) Code() string {
	var result []string
	for _, child := range x.tests {
		result = append(result, "\t\t"+child.toString())
	}
	return strings.Join(result, "\n")
}

func (x UnitTest) toString() string {
	return fmt.Sprintf(
		"testNamedKey(t, fn, %s, `%s`, `%s`, %v)",
		x.value,
		x.expected,
		fmt.Sprintf("fn("+x.value+", %v)", x.ordinal),
		x.ordinal,
	)
}

func Sanitize(input string) string {
	var result string
	for _, char := range input {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z':
			result += string(char)
		}
	}
	return result
}

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
			if 1 == len(data) {
				return padding + "return \"other\"\n"
			}
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
	if 1 == len(data) {
		return rule2code("other", data, ptr_keys, padding)
	}
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

func splitValues(input string) []string {
	var result []string

	pos := -1
	for idx, char := range input {
		switch {
		case (char >= '0' && char <= '9') || '.' == char:
			if -1 == pos {
				pos = idx
			}

		// Inutile de générer un interval lorsque l'on rencontre '~' :)
		case ' ' == char || ',' == char || '~' == char:
			if -1 != pos {
				result = append(result, input[pos:idx])
				pos = -1
			}
		}
	}

	if -1 != pos {
		result = append(result, input[pos:])
	}
	return result
}

func pattern2test(expected, input string, ordinal bool) []Test {
	var result []Test

	patterns := strings.Split(input, "@")
	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, "integer") {
			for _, value := range splitValues(pattern[8:]) {
				result = append(result, UnitTest{ordinal, expected, value})
			}
		} else if strings.HasPrefix(pattern, "decimal") {
			for _, value := range splitValues(pattern[8:]) {
				result = append(result, UnitTest{ordinal, expected, value})
			}
		}
	}
	return result
}

func map2test(ordinals, plurals map[string]string) []Test {
	var result []Test

	for _, rule := range []string{"one", "two", "few", "many", "zero", "other"} {
		if input, ok := ordinals["pluralRule-count-"+rule]; ok {
			result = append(result, pattern2test(rule, input, true)...)
		}

		if input, ok := plurals["pluralRule-count-"+rule]; ok {
			result = append(result, pattern2test(rule, input, false)...)
		}
	}
	return result
}

func culture2code(ordinals, plurals map[string]string, padding string) (string, string, []Test) {
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
	tests := map2test(ordinals, plurals)

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
	}
	return vars, code, tests
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

func createGoFiles(headers string, ptr_plurals, ptr_ordinals *map[string]map[string]string) error {
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

	var items []Source
	var tests []Source

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

			vars, code, unit_tests := culture2code(ordinals, plurals, "\t\t")
			items = append(items, FuncSource{culture, vars, code})

			fmt.Println(" \u2713")

			if len(unit_tests) > 0 {
				tests = append(tests, UnitTestSource{culture, unit_tests})
			}
		}
	}

	if len(tests) > 0 {
		err := createSource("plural_test.tmpl", "plural/func_test.go", headers, tests)
		if nil != err {
			return err
		}
	}
	return createSource("plural.tmpl", "plural/func.go", headers, items)
}

func createSource(tmpl_filepath, dest_filepath, headers string, items []Source) error {
	source, err := template.ParseFiles(tmpl_filepath)
	if nil != err {
		return err
	}

	file, err := os.Create(dest_filepath)
	if nil != err {
		return err
	}
	defer file.Close()

	return source.Execute(file, struct {
		Headers   string
		Timestamp string
		Items     []Source
	}{
		headers,
		time.Now().Format(time.RFC1123Z),
		items,
	})
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

			err = createGoFiles(headers, &plurals, &ordinals)
			if nil != err {
				fmt.Println(err, "(╯°□°）╯︵ ┻━┻")
			} else {
				fmt.Println("Succeed (ッ)")
			}
		}
	}
}
