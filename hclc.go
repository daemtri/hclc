package hclc

import (
	"errors"
	"fmt"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/hcl/hcl/printer"
	"github.com/hashicorp/hcl/hcl/token"
	"io/ioutil"
	"os"
	"reflect"
)

// File 表示一个配置文件
type File struct {
	List *ast.ObjectList
}

func mergeComment(a, b *ast.CommentGroup) *ast.CommentGroup {
	if a == nil {
		return b
	} else if b == nil {
		return a
	}

LOOP:
	for _, commentB := range b.List {
		for _, commentA := range a.List {
			if commentA.Text == commentB.Text {
				continue LOOP
			}
		}
		a.List = append(a.List, commentB)
	}
	return &ast.CommentGroup{List: a.List}
}

func keyEqual(left []*ast.ObjectKey, right []*ast.ObjectKey) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].Token.Text != right[i].Token.Text {
			return false
		}
	}
	return true
}

func (f *File) put(item *ast.ObjectItem) error {
	for i := range f.List.Items {
		if keyEqual(f.List.Items[i].Keys, item.Keys) {
			old := f.List.Items[i]
			old.Assign = item.Assign
			old.LeadComment = mergeComment(old.LeadComment, item.LeadComment)
			old.LineComment = mergeComment(old.LineComment, item.LineComment)
			old.Val = item.Val

			return nil
		}
	}
	f.List.Add(item)
	return nil
}

// Set 发射
func (f *File) Set(key string, value interface{}) error {
	if key == "" {
		return errors.New("empty section key")
	}
	node, _, err := encode(reflect.ValueOf(value))
	if err != nil {
		return err
	}
	nodeKey := &ast.ObjectKey{
		Token: token.Token{
			Type: token.IDENT,
			Text: key,
		},
	}
	item := &ast.ObjectItem{
		Keys: []*ast.ObjectKey{nodeKey},
		Val:  node,
	}

	return f.put(item)
}

// Get 映射到文件
func (f *File) Get(key string, value interface{}) error {
	if key == "" {
		return errors.New("empty section key")
	}
	ret := f.List.Filter(key)
	if len(ret.Items) == 0 {
		return fmt.Errorf("section %s not found", key)
	} else if len(ret.Items) > 1 {
		return fmt.Errorf("section %s set multi times", key)
	}

	return DecodeObject(value, ret.Items[0])
}

// GetList 映射配置到结构体之中
func (f *File) GetList(value interface{}) error {
	return DecodeObject(value, f.List)
}

// SetList 映射到列表之中
func (f *File) SetList(value interface{}) error {
	node, _, err := encode(reflect.ValueOf(value))
	if err != nil {
		return err
	}

	ot, ok := node.(*ast.ObjectType)
	if !ok {
		return errors.New("not list type")
	}
	for _, item := range ot.List.Items {
		if err := f.put(item); err != nil {
			return err
		}
	}

	return nil
}

// Exists key是否存在
func (f *File) Exists(key string) bool {
	list := f.List.Filter(key)
	return len(list.Items) > 0
}

// SaveToFile 保存到文件
func SaveToFile(filename string, f *File) error {
	if _, err := positionNodes(f.List, startingCursor, 2); err != nil {
		return err
	}
	cfgFile, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.ModePerm)
	if err != nil {
		return err
	}
	printer.Fprint(cfgFile, f.List)
	return nil
}

// LooseLoad 若加载
func LooseLoad(filename string) (*File, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{
				List: &ast.ObjectList{
					Items: make([]*ast.ObjectItem, 0, 0),
				},
			}, nil
		}
		return nil, err
	}
	astFile, err := hcl.ParseBytes(data)
	if err != nil {
		return nil, err
	}
	f := &File{
		List: astFile.Node.(*ast.ObjectList),
	}
	return f, nil
}
