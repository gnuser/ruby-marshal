package rbmarshal

import (
	"bufio"
	"errors"
	"io"
	"reflect"
	// "fmt"
)

const (
	SUPPORTED_MAJOR_VERSION = 4
	SUPPORTED_MINOR_VERSION = 8

	NIL_SIGN         = '0'
	TRUE_SIGN        = 'T'
	FALSE_SIGN       = 'F'
	FIXNUM_SIGN      = 'i'
	RAWSTRING_SIGN   = '"'
	SYMBOL_SIGN      = ':'
	SYMBOL_LINK_SIGN = ';'
	OBJECT_SIGN      = 'o'
	OBJECT_LINK_SIGN = '@'
	ARRAY_SIGN       = '['
	IVAR_SIGN        = 'I'
	HASH_SIGN        = '{'
	BIGNUM_SIGN      = 'l'
	REGEXP_SIGN      = '/'
	CLASS_SIGN       = 'c'
	MODULE_SIGN      = 'm'
)

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: bufio.NewReader(r)}
}

type Decoder struct {
	r       *bufio.Reader
	objects []interface{}
	symbols []string
}

func (d *Decoder) unmarshal() (result interface{}) {
	typ, _ := d.r.ReadByte()
	// fmt.Printf("type : %x, ", typ)
	switch typ {
	case NIL_SIGN: // 0 - nil
		result = nil
	case TRUE_SIGN: // T - true
		result = true
	case FALSE_SIGN: // F - false
		result = false
	case FIXNUM_SIGN: // i - integer
		result = d.parseInt()
	case RAWSTRING_SIGN: // " - string
		result = d.parseString()
	case SYMBOL_SIGN: // : - symbol
		result = d.parseSymbol()
	case SYMBOL_LINK_SIGN: // ; - symbol symlink
		result = d.parseSymLink()
	case OBJECT_LINK_SIGN: // @ - object link
		panic("not supported.")
	case IVAR_SIGN: // I - IVAR (encoded string or regexp)
		result = d.parseIvar()
	case ARRAY_SIGN: // [ - array
		result = d.parseArray()
	case OBJECT_SIGN: // o - object
		panic("not supported.")
	case HASH_SIGN: // { - hash
		result = d.parseHash()
	case BIGNUM_SIGN: // l - bignum
		panic("not supported.")
	case REGEXP_SIGN: // / - regexp
		panic("not supported.")
	case CLASS_SIGN: // c - class
		panic("not supported.")
	case MODULE_SIGN: // m -module
		panic("not supported.")
	default:
		result = nil
	}
	// fmt.Printf("result: %v, %x \n", result, result)
	return
}


func (d *Decoder) parseInt() int {
	var result int
	b, _ := d.r.ReadByte()
	c := int(int8(b))
	if c == 0 {
		return 0
	} else if 5 < c && c < 128 {
		return c - 5
	} else if -129 < c && c < -5 {
		return c + 5
	}
	cInt8 := int8(b)
	if cInt8 > 0 {
		result = 0
		for i := int8(0); i < cInt8; i++ {
			n, _ := d.r.ReadByte()
			result |= int(uint(n) << (8 * uint(i)))
		}
	} else {
		result = -1
		c = -c
		for i := 0; i < c; i++ {
			n, _ := d.r.ReadByte()
			result &= ^(0xff << uint(8*i))
			result |= int(n) << uint(8*i)
		}
	}
	return result
}

func (d *Decoder) parseSymbol() string {
	symbol := d.parseString()
	d.symbols = append(d.symbols, symbol)
	return symbol
}

func (d *Decoder) parseSymLink() string {
	index := d.parseInt()
	return d.symbols[index]
}

func (d *Decoder) parseObjectLink() interface{} {
	index := d.parseInt()
	return d.objects[index]
}

func (d *Decoder) parseString() string {
	len := d.parseInt()
	str := make([]byte, len)
	d.r.Read(str)
	return string(str)
}

type iVar struct {
	str string
}

func (d *Decoder) parseIvar() string {
	str := d.unmarshal()

	symbolCharLen := d.parseInt()

	if symbolCharLen == 1 {
		symbol := d.unmarshal().(string) // :E
		_ = d.unmarshal()                // T
		d.symbols = append(d.symbols, symbol)
	}

	strString := str.(string)
	ivar := iVar{strString}
	d.objects = append(d.objects, ivar)
	return strString
}

func (d *Decoder) parseArray() interface{} {
	size := d.parseInt()
	arr := make([]interface{}, size)

	for i := 0; i < int(size); i++ {
		arr[i] = d.unmarshal()
	}
	return arr
}

func (d *Decoder) parseHash() interface{} {
	size := d.parseInt()
	hash := make(map[string]interface{}, size)

	for i := 0; i < int(size); i++ {
		key := d.unmarshal()
		value := d.unmarshal()
		hash[key.(string)] = value
	}

	return hash
}

func (d *Decoder) Decode(v interface{}) error {
	major, err := d.r.ReadByte()
	minor, err := d.r.ReadByte()

	if err != nil {
		return errors.New("cant decode MAJOR, MINOR version")
	}
	if major != SUPPORTED_MAJOR_VERSION || minor > SUPPORTED_MINOR_VERSION {
		return errors.New("unsupported marshal version")
	}

	val := reflect.ValueOf(v)

	if val.Kind() != reflect.Ptr {
		return errors.New("pointer need.")
	}

	r := d.unmarshal()
	if r == nil {
		v = nil
		return nil
	}

	if val.Elem().Kind() == reflect.Struct {
		MapToStruct(r, v)
	} else {
		val.Elem().Set(reflect.ValueOf(r))
	}

	return nil
}

func MapToStruct(mi interface{}, o interface{}) {
	oValue := reflect.ValueOf(o).Elem()
	oType := reflect.TypeOf(o).Elem()
	m := mi.(map[string]interface{})

	for i := 0; i < oValue.NumField(); i++ {
		field := oType.Field(i)
		val := m[field.Tag.Get("ruby")]
		if val == nil {
			continue
		}

		if mm, ok := val.(map[string]interface{}); ok {
			if fieldObj := oValue.Field(i); fieldObj.Kind() == reflect.Ptr {
				if fieldObj.IsNil() {
					newObj := reflect.New(fieldObj.Type().Elem())
					fieldObj.Set(newObj)
				}
				MapToStruct(mm, fieldObj.Interface())
			} else {
				MapToStruct(mm, fieldObj.Addr().Interface())
			}
		} else {
			oValue.Field(i).Set(reflect.ValueOf(val))
		}
	}

}

type Encoder struct {
	w            *bufio.Writer
	symbols      map[string]int
	symbolsIndex int
	objects      map[*interface{}]int
	objectsIndex int
	stringObj    iVar
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w:            bufio.NewWriter(w),
		symbols:      map[string]int{},
		symbolsIndex: 0,
		objects:      map[*interface{}]int{},
		objectsIndex: 0,
	}
}

func (e *Encoder) Encode(v interface{}) error {
	if _, err := e.w.Write([]byte{SUPPORTED_MAJOR_VERSION, SUPPORTED_MINOR_VERSION}); err != nil {
		return err
	}

	e.marshal(v)

	e.w.Flush()
	return nil
}

func (e *Encoder) marshal(v interface{}) error {
	vKind := reflect.TypeOf(v).Kind()
	val := reflect.ValueOf(v)
	typ := reflect.TypeOf(v)

	if vKind == reflect.Ptr {
		val = val.Elem()
		typ = typ.Elem()
	}

	// fmt.Printf("typ: %v\n", typ)
	switch typ.Kind() {
	case reflect.Bool:
		return e.encBool(val.Bool())
	case reflect.Int:
		e.w.WriteByte(FIXNUM_SIGN)
		return e.encInt(int(val.Int()))
	case reflect.String:
		return e.encString(val.String())
	case reflect.Slice:
		return e.encSlice(v)
	case reflect.Map:
		return e.encMap(v)
	}
	return nil
}

func (e *Encoder) encBool(val bool) error {
	// fmt.Printf("enc bool , %v, treu: %x, false: %x\n", val, TRUE_SIGN, FALSE_SIGN)
	if val {
		return e.w.WriteByte(TRUE_SIGN)
	}
	return e.w.WriteByte(FALSE_SIGN)
}

func (e *Encoder) encInt(i int) error {
	var len int

	if i == 0 {
		return e.w.WriteByte(0)
	} else if 0 < i && i < 123 {
		return e.w.WriteByte(byte(i + 5))
	} else if -124 < i && i <= -1 {
		return e.w.WriteByte(byte(i - 5))
	} else if 122 < i && i <= 0xff {
		len = 1
	} else if 0xff < i && i <= 0xffff {
		len = 2
	} else if 0xffff < i && i <= 0xffffff {
		len = 3
	} else if 0xffffff < i && i <= 0x3fffffff {
		//for compatibility with 32bit Ruby, Fixnum should be less than 1073741824
		len = 4
	} else if -0x100 <= i && i < -123 {
		len = -1
	} else if -0x10000 <= i && i < -0x100 {
		len = -2
	} else if -0x1000000 <= i && i < -0x100000 {
		len = -3
	} else if -0x40000000 <= i && i < -0x1000000 {
		//for compatibility with 32bit Ruby, Fixnum should be greater than -1073741825
		len = -4
	}

	if err := e.w.WriteByte(byte(len)); err != nil {
		return err
	}
	if len < 0 {
		len = -len
	}

	for c := 0; c < len; c++ {
		if err := e.w.WriteByte(byte(i >> uint(8*c) & 0xff)); err != nil {
			return err
		}
	}

	return nil
}

func (e *Encoder) _encRawString(str string) error {
	// | len (Fixnum) | stirng |
	if err := e.encInt(len(str)); err != nil {
		return err
	}

	_, err := e.w.WriteString(str)
	return err
}

func (e *Encoder) encString(str string) error {
	// fmt.Printf("enc string :%v, %x\n", str, str)
	// | I | " | RawString( string ) | FixNum( 1 ) | Symbol( E ) | True |
	if _, err := e.w.Write([]byte{IVAR_SIGN, RAWSTRING_SIGN}); err != nil {
		return err
	}
	if err := e._encRawString(str); err != nil {
		return err
	}
	if err := e.encInt(1); err != nil {
		return err
	}
	if err := e._encSymbol("E"); err != nil {
		return err
	}
	return e.encBool(true)
}

func (e *Encoder) _encSymbol(str string) error {
	if index, ok := e.symbols[str]; ok {
		if err := e.w.WriteByte(SYMBOL_LINK_SIGN); err != nil {
			return err
		}
		return e.encInt(index)
	}

	e.symbols[str] = e.symbolsIndex
	e.symbolsIndex++

	if err := e.w.WriteByte(SYMBOL_SIGN); err != nil {
		return err
	}
	if err := e.encInt(len(str)); err != nil {
		return err
	}
	_, err := e.w.WriteString(str)
	return err
}

func (e *Encoder) encSlice(arr interface{}) error {
	// fmt.Println("enc slice")
	e.w.WriteByte(ARRAY_SIGN)
	a := arr.([]interface{})
	e.encInt(len(a))
	for _, item := range a {
		err := e.marshal(item)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *Encoder) encMap(v interface{}) error {
	m := v.(map[string]interface{})
	e.w.WriteByte(HASH_SIGN)
	e.encInt(len(m))
	for key, value := range m {
		err := e.marshal(key)
		if err != nil {
			return err
		}
		err = e.marshal(value)
		if err != nil {
			return err
		}
	}
	return nil
}
