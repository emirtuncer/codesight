package walkers

import (
	"context"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
	"github.com/emirtuncer/codesight/internal/parser"
)

func parseTS(t *testing.T, source string) (*sitter.Tree, []byte) {
	t.Helper()
	p := sitter.NewParser()
	p.SetLanguage(typescript.GetLanguage())
	src := []byte(source)
	tree, err := p.ParseCtx(context.Background(), nil, src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return tree, src
}

func TestTSWalkerDetectsDecorators(t *testing.T) {
	source := `@Component({
  selector: 'app-root'
})
class AppComponent {
  title: string = 'hello';
}
`
	tree, src := parseTS(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "AppComponent", Kind: "class", QualifiedName: "AppComponent"},
	}

	w := &TSWalker{}
	symbols, _ := w.Walk(tree, src, inputSymbols)

	found := false
	for _, sym := range symbols {
		if sym.Name == "AppComponent" && sym.Kind == "class" {
			if sym.Metadata != nil && sym.Metadata["decorators"] == "@Component" {
				found = true
			} else {
				t.Errorf("expected decorators='@Component', got metadata=%v", sym.Metadata)
			}
		}
	}
	if !found {
		t.Error("should detect @Component decorator on AppComponent class")
	}
}

func TestTSWalkerDetectsImplements(t *testing.T) {
	source := `export interface Serializable {
  toJSON(): string;
}

export class BaseEntity implements Serializable {
  id: string = '';

  toJSON(): string {
    return JSON.stringify(this);
  }
}
`
	tree, src := parseTS(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "Serializable", Kind: "interface", QualifiedName: "Serializable"},
		{Name: "BaseEntity", Kind: "class", QualifiedName: "BaseEntity"},
		{Name: "toJSON", Kind: "method", QualifiedName: "toJSON"},
	}

	w := &TSWalker{}
	_, deps := w.Walk(tree, src, inputSymbols)

	hasImplements := false
	for _, d := range deps {
		if d.Kind == "implements" && d.TargetName == "Serializable" && d.SourceSymbol == "BaseEntity" {
			hasImplements = true
		}
	}
	if !hasImplements {
		t.Error("should detect that BaseEntity implements Serializable")
	}
}

func TestTSWalkerDetectsExtends(t *testing.T) {
	source := `class Animal {
  name: string = '';
}

class Dog extends Animal {
  breed: string = '';
}
`
	tree, src := parseTS(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "Animal", Kind: "class", QualifiedName: "Animal"},
		{Name: "Dog", Kind: "class", QualifiedName: "Dog"},
	}

	w := &TSWalker{}
	_, deps := w.Walk(tree, src, inputSymbols)

	hasExtends := false
	for _, d := range deps {
		if d.Kind == "extends" && d.TargetName == "Animal" && d.SourceSymbol == "Dog" {
			hasExtends = true
		}
	}
	if !hasExtends {
		t.Error("should detect that Dog extends Animal")
	}
}

func TestTSWalkerEnrichesExportStatus(t *testing.T) {
	source := `export function createUser(name: string): User {
  return { name };
}

function helperFunc(): void {}
`
	tree, src := parseTS(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "createUser", Kind: "function", QualifiedName: "createUser", LineStart: 1, LineEnd: 3},
		{Name: "helperFunc", Kind: "function", QualifiedName: "helperFunc", LineStart: 5, LineEnd: 5},
	}

	w := &TSWalker{}
	symbols, _ := w.Walk(tree, src, inputSymbols)

	for _, sym := range symbols {
		switch sym.Name {
		case "createUser":
			if !sym.IsExported {
				t.Error("createUser should be marked as exported")
			}
			if sym.Visibility != "public" {
				t.Errorf("createUser visibility should be 'public', got %q", sym.Visibility)
			}
		case "helperFunc":
			if sym.IsExported {
				t.Error("helperFunc should NOT be marked as exported")
			}
		}
	}
}

func TestTSWalkerAssignsMethodParent(t *testing.T) {
	source := `class UserService {
  addUser(user: User): void {
    console.log(user);
  }
}
`
	tree, src := parseTS(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "UserService", Kind: "class", QualifiedName: "UserService", LineStart: 1, LineEnd: 5},
		{Name: "addUser", Kind: "method", QualifiedName: "addUser", LineStart: 2, LineEnd: 4},
	}

	w := &TSWalker{}
	symbols, _ := w.Walk(tree, src, inputSymbols)

	for _, sym := range symbols {
		if sym.Name == "addUser" {
			if sym.ParentName != "UserService" {
				t.Errorf("addUser parent should be 'UserService', got %q", sym.ParentName)
			}
			if sym.QualifiedName != "UserService.addUser" {
				t.Errorf("addUser qualified name should be 'UserService.addUser', got %q", sym.QualifiedName)
			}
		}
	}
}
