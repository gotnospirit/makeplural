package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strconv"
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

	Op struct {
		previous_logic, left, operator, right, next_logic string
	}
)

func (x FuncSource) Culture() string {
	return x.culture
}

func (x FuncSource) CultureId() string {
	return sanitize(x.culture)
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
	return sanitize(x.culture)
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

func sanitize(input string) string {
	var result string
	for _, char := range input {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z':
			result += string(char)
		}
	}
	return result
}

func (x Op) conditions() []string {
	var result []string

	conditions := strings.Split(x.right, ",")
	for _, condition := range conditions {
		pos := strings.Index(condition, "..")

		if -1 != pos {
			lower_bound, upper_bound := condition[:pos], condition[pos+2:]
			lb, _ := strconv.Atoi(lower_bound)
			ub, _ := strconv.Atoi(upper_bound)

			r := rangeCondition(x.left, lb, ub, x.operator)
			result = append(result, r...)
		} else {
			result = append(result, fmt.Sprintf("%s %s %s", x.left, x.operator, condition))
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

func rangeCondition(varname string, lower, upper int, operator string) []string {
	var result []string
	for i := lower; i <= upper; i++ {
		result = append(result, fmt.Sprintf("%s %s %d", varname, operator, i))
	}
	return result
}

func pattern2code(input string, ptr_vars *[]string) []string {
	left, short, operator, logic := "", "", "", ""

	var ops []Op
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
				short = toVar(left, ptr_vars)
			}

		case '!':
			left, operator, buf = buf, "!=", ""
			short = toVar(left, ptr_vars)
		}

		if "" != buf {
			pos := strings.Index(buf, "and")

			if -1 != pos {
				ops = append(ops, Op{logic, short, operator, buf[:pos], "AND"})
				buf, left, operator, logic = "", "", "", "AND"
			} else {
				pos = strings.Index(buf, "or")

				if -1 != pos {
					ops = append(ops, Op{logic, short, operator, buf[:pos], "OR"})
					buf, left, operator, logic = "", "", "", "OR"
				}
			}
		}
	}

	if "" != buf {
		ops = append(ops, Op{logic, short, operator, buf, ""})
	}

	if 1 == len(ops) {
		conditions := ops[0].conditions()
		if "==" == ops[0].operator {
			return conditions
		} else {
			return []string{strings.Join(conditions, " && ")}
		}
	}

	var result []string
	var buffer []string

	buffer_length := 0
	for _, o := range ops {
		conditions := o.conditions()
		logic = o.previous_logic
		nextLogic := o.next_logic
		operator := o.operator

		if "OR" == logic && buffer_length > 0 {
			result = append(result, strings.Join(buffer, ", "))
			buffer = []string{}
			buffer_length = 0
		}

		if ("" == logic && "OR" == nextLogic) || ("OR" == logic && "OR" == nextLogic) || ("OR" == logic && "" == nextLogic) {
			if "==" == operator {
				buffer = append(buffer, conditions...)
			} else {
				buffer = append(buffer, strings.Join(conditions, " && "))
			}
			buffer_length = len(buffer)
		} else if "AND" == logic && ("AND" == nextLogic || "" == nextLogic) {
			if "==" == operator {
				buffer[buffer_length-1] += " && " + joinOr(conditions)
			} else {
				buffer[buffer_length-1] += " && " + strings.Join(conditions, " && ")
			}
		} else if "" == logic && "AND" == nextLogic {
			if "==" == operator {
				buffer = append(buffer, joinOr(conditions))
			} else {
				buffer = append(buffer, strings.Join(conditions, " && "))
			}
			buffer_length = len(buffer)
		} else if "OR" == logic && "AND" == nextLogic {
			if "==" == operator {
				if len(conditions) > 1 {
					buffer = append(buffer, joinOr(conditions))
				} else {
					buffer = append(buffer, conditions...)
				}
			} else {
				buffer = append(buffer, strings.Join(conditions, " && "))
			}
			buffer_length = len(buffer)
		} else if "AND" == logic && "OR" == nextLogic {
			if "==" == operator {
				buffer[buffer_length-1] += " && " + joinOr(conditions)
			} else {
				buffer[buffer_length-1] += " && " + strings.Join(conditions, " && ")
			}
		}
	}

	if len(buffer) > 0 {
		if "OR" == logic {
			result = append(result, buffer...)
		} else {
			result = append(result, strings.Join(buffer, " && "))
		}
	}
	return result
}

func joinOr(data []string) string {
	if len(data) > 1 {
		return "(" + strings.Join(data, " || ") + ")"
	}
	return data[0]
}

func rule2code(key string, data map[string]string, ptr_vars *[]string, padding string) string {
	if input, ok := data["pluralRule-count-"+key]; ok {
		result := ""

		if "other" == key {
			if 1 == len(data) {
				return padding + "return \"other\"\n"
			}
			result += padding + "default:\n"
		} else {
			cases := pattern2code(input, ptr_vars)
			result += "\n" + padding + "case " + strings.Join(cases, ", ") + ":\n"
		}
		result += padding + "\treturn \"" + key + "\"\n"
		return result
	}
	return ""
}

func map2code(data map[string]string, ptr_vars *[]string, padding string) string {
	if 1 == len(data) {
		return rule2code("other", data, ptr_vars, padding)
	}
	result := padding + "switch {\n"
	result += rule2code("other", data, ptr_vars, padding)
	result += rule2code("zero", data, ptr_vars, padding)
	result += rule2code("one", data, ptr_vars, padding)
	result += rule2code("two", data, ptr_vars, padding)
	result += rule2code("few", data, ptr_vars, padding)
	result += rule2code("many", data, ptr_vars, padding)
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
				result = append(result, UnitTest{ordinal, expected, "\"" + value + "\""})
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
	var vars []string

	if nil == ordinals {
		code = map2code(plurals, &vars, padding)
	} else {
		code = padding + "if ordinal {\n"
		code += map2code(ordinals, &vars, padding+"\t")
		code += padding + "}\n\n"
		code += map2code(plurals, &vars, padding)
	}
	tests := map2test(ordinals, plurals)

	str_vars := ""
	max := len(vars)

	if max > 0 {
		// http://unicode.org/reports/tr35/tr35-numbers.html#Operands
		//
		// Symbol	Value
		// n	    absolute value of the source number (integer and decimals).
		// i	    integer digits of n.
		// v	    number of visible fraction digits in n, with trailing zeros.
		// w	    number of visible fraction digits in n, without trailing zeros.
		// f	    visible fractional digits in n, with trailing zeros.
		// t	    visible fractional digits in n, without trailing zeros.
		var_f := varname('f', vars)
		var_i := varname('i', vars)
		var_n := varname('n', vars)
		var_v := varname('v', vars)
		var_t := varname('t', vars)
		var_w := varname('w', vars)

		if "_" != var_f || "_" != var_v || "_" != var_t || "_" != var_w {
			str_vars += padding + fmt.Sprintf("%s, %s, %s, %s, %s, %s := finvtw(value)\n", var_f, var_i, var_n, var_v, var_t, var_w)
		} else {
			if "_" != var_n {
				if "_" != var_i {
					str_vars += padding + "flt := float(value)\n"
					str_vars += padding + "n := math.Abs(flt)\n"
					str_vars += padding + "i := int64(flt)\n"
				} else {
					str_vars += padding + "n := math.Abs(float(value))\n"
				}
			} else if "_" != var_i {
				str_vars += padding + "i := int64(float(value))\n"
			}
		}

		for i := 0; i < max; i += 2 {
			k := vars[i]
			v := vars[i+1]

			if k != v {
				str_vars += padding + k + " := " + v + "\n"
			}
		}
	}
	return str_vars, code, tests
}

func addVar(varname, expr string, ptr_vars *[]string) string {
	exists := false
	for i := 0; i < len(*ptr_vars); i += 2 {
		if (*ptr_vars)[i] == varname {
			exists = true
			break
		}
	}

	if !exists {
		*ptr_vars = append(*ptr_vars, varname, expr)
	}
	return varname
}

func toVar(expr string, ptr_vars *[]string) string {
	var varname string

	if pos := strings.Index(expr, "%"); -1 != pos {
		k, v := expr[:pos], expr[pos+1:]
		varname = k + v
		if "n" == k {
			expr = "mod(n, " + v + ")"
		} else {
			expr = k + " % " + v
		}
	} else {
		varname = expr
	}
	return addVar(varname, expr, ptr_vars)
}

func varname(char uint8, vars []string) string {
	for i := 0; i < len(vars); i += 2 {
		if char == vars[i][0] {
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
