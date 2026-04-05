package walkers

import (
	"context"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/csharp"
	"github.com/emirtuncer/codesight/internal/parser"
)

func parseCSharp(t *testing.T, source string) (*sitter.Tree, []byte) {
	t.Helper()
	p := sitter.NewParser()
	p.SetLanguage(csharp.GetLanguage())
	src := []byte(source)
	tree, err := p.ParseCtx(context.Background(), nil, src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return tree, src
}

func TestCSharpWalkerNamespacePrefixing(t *testing.T) {
	source := `namespace MyApp.Services
{
    public class UserService
    {
        public void DoWork() { }
    }
}
`
	tree, src := parseCSharp(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "UserService", Kind: "class", QualifiedName: "UserService", LineStart: 3, LineEnd: 6},
		{Name: "DoWork", Kind: "method", QualifiedName: "DoWork", LineStart: 5, LineEnd: 5},
	}

	w := &CSharpWalker{}
	symbols, _ := w.Walk(tree, src, inputSymbols)

	for _, sym := range symbols {
		switch sym.Name {
		case "UserService":
			expected := "MyApp.Services.UserService"
			if sym.QualifiedName != expected {
				t.Errorf("UserService qualified name: got %q, want %q", sym.QualifiedName, expected)
			}
		case "DoWork":
			expected := "MyApp.Services.UserService.DoWork"
			if sym.QualifiedName != expected {
				t.Errorf("DoWork qualified name: got %q, want %q", sym.QualifiedName, expected)
			}
			if sym.ParentName != "UserService" {
				t.Errorf("DoWork parent: got %q, want %q", sym.ParentName, "UserService")
			}
		}
	}
}

func TestCSharpWalkerInheritanceDetection(t *testing.T) {
	source := `namespace MyApp
{
    public interface IUserService
    {
    }

    public class UserService : IUserService
    {
    }
}
`
	tree, src := parseCSharp(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "IUserService", Kind: "interface", QualifiedName: "IUserService", LineStart: 3, LineEnd: 5},
		{Name: "UserService", Kind: "class", QualifiedName: "UserService", LineStart: 7, LineEnd: 9},
	}

	w := &CSharpWalker{}
	_, deps := w.Walk(tree, src, inputSymbols)

	foundImplements := false
	for _, dep := range deps {
		if dep.Kind == "implements" && dep.TargetName == "IUserService" && dep.SourceSymbol == "UserService" {
			foundImplements = true
		}
	}
	if !foundImplements {
		t.Error("should detect UserService implements IUserService")
	}
}

func TestCSharpWalkerVisibilityExtraction(t *testing.T) {
	source := `namespace MyApp
{
    public class MyClass
    {
        public void PublicMethod() { }
        private void PrivateMethod() { }
        protected void ProtectedMethod() { }
        internal void InternalMethod() { }
    }
}
`
	tree, src := parseCSharp(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "MyClass", Kind: "class", QualifiedName: "MyClass", LineStart: 3, LineEnd: 9},
		{Name: "PublicMethod", Kind: "method", QualifiedName: "PublicMethod", LineStart: 5, LineEnd: 5},
		{Name: "PrivateMethod", Kind: "method", QualifiedName: "PrivateMethod", LineStart: 6, LineEnd: 6},
		{Name: "ProtectedMethod", Kind: "method", QualifiedName: "ProtectedMethod", LineStart: 7, LineEnd: 7},
		{Name: "InternalMethod", Kind: "method", QualifiedName: "InternalMethod", LineStart: 8, LineEnd: 8},
	}

	w := &CSharpWalker{}
	symbols, _ := w.Walk(tree, src, inputSymbols)

	for _, sym := range symbols {
		switch sym.Name {
		case "MyClass":
			if sym.Visibility != "public" {
				t.Errorf("MyClass visibility: got %q, want %q", sym.Visibility, "public")
			}
			if !sym.IsExported {
				t.Error("MyClass should be exported")
			}
		case "PublicMethod":
			if sym.Visibility != "public" {
				t.Errorf("PublicMethod visibility: got %q, want %q", sym.Visibility, "public")
			}
			if !sym.IsExported {
				t.Error("PublicMethod should be exported")
			}
		case "PrivateMethod":
			if sym.Visibility != "private" {
				t.Errorf("PrivateMethod visibility: got %q, want %q", sym.Visibility, "private")
			}
			if sym.IsExported {
				t.Error("PrivateMethod should NOT be exported")
			}
		case "ProtectedMethod":
			if sym.Visibility != "protected" {
				t.Errorf("ProtectedMethod visibility: got %q, want %q", sym.Visibility, "protected")
			}
		case "InternalMethod":
			if sym.Visibility != "internal" {
				t.Errorf("InternalMethod visibility: got %q, want %q", sym.Visibility, "internal")
			}
		}
	}
}

func TestCSharpWalkerStructAndRecordInheritance(t *testing.T) {
	source := `namespace MyApp
{
    public interface IDisposable
    {
    }

    public interface IHandler<T>
    {
    }

    public struct MyStruct : IDisposable
    {
    }

    public record MyRecord : IHandler<string>
    {
    }
}
`
	tree, src := parseCSharp(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "IDisposable", Kind: "interface", QualifiedName: "IDisposable", LineStart: 3, LineEnd: 5},
		{Name: "IHandler", Kind: "interface", QualifiedName: "IHandler", LineStart: 7, LineEnd: 9},
		{Name: "MyStruct", Kind: "struct", QualifiedName: "MyStruct", LineStart: 11, LineEnd: 13},
		{Name: "MyRecord", Kind: "class", QualifiedName: "MyRecord", LineStart: 15, LineEnd: 17},
	}

	w := &CSharpWalker{}
	symbols, deps := w.Walk(tree, src, inputSymbols)

	// Verify namespace enrichment for struct
	for _, sym := range symbols {
		if sym.Name == "MyStruct" && sym.QualifiedName != "MyApp.MyStruct" {
			t.Errorf("MyStruct qualified name: got %q, want %q", sym.QualifiedName, "MyApp.MyStruct")
		}
	}

	// Verify struct implements IDisposable
	foundStruct := false
	for _, dep := range deps {
		if dep.Kind == "implements" && dep.SourceSymbol == "MyStruct" && dep.TargetName == "IDisposable" {
			foundStruct = true
		}
	}
	if !foundStruct {
		t.Error("should detect MyStruct implements IDisposable")
	}

	// Verify record implements IHandler (generic)
	foundRecord := false
	for _, dep := range deps {
		if dep.Kind == "implements" && dep.SourceSymbol == "MyRecord" && dep.TargetName == "IHandler" {
			foundRecord = true
		}
	}
	if !foundRecord {
		t.Error("should detect MyRecord implements IHandler<string>")
	}
}

func TestCSharpWalkerGenericInterfaceImplements(t *testing.T) {
	source := `namespace MyApp.CQRS
{
    public interface ICommandHandler<TCommand, TResult>
    {
    }

    public class ProcessChatHandler : ICommandHandler<ProcessChatCommand, Result<ChatResponse>>
    {
    }

    public class MultiBase : BaseClass, IFoo, ICommandHandler<string, int>
    {
    }
}
`
	tree, src := parseCSharp(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "ICommandHandler", Kind: "interface", QualifiedName: "ICommandHandler", LineStart: 3, LineEnd: 5},
		{Name: "ProcessChatHandler", Kind: "class", QualifiedName: "ProcessChatHandler", LineStart: 7, LineEnd: 9},
		{Name: "MultiBase", Kind: "class", QualifiedName: "MultiBase", LineStart: 11, LineEnd: 13},
	}

	w := &CSharpWalker{}
	_, deps := w.Walk(tree, src, inputSymbols)

	// ProcessChatHandler should implement ICommandHandler (not Result or ChatResponse)
	found := false
	for _, dep := range deps {
		if dep.Kind == "implements" && dep.SourceSymbol == "ProcessChatHandler" && dep.TargetName == "ICommandHandler" {
			found = true
		}
		// Result and ChatResponse should NOT appear as base types
		if dep.TargetName == "Result" || dep.TargetName == "ChatResponse" || dep.TargetName == "ProcessChatCommand" {
			t.Errorf("type argument %q should not appear as a base type dependency", dep.TargetName)
		}
	}
	if !found {
		t.Error("should detect ProcessChatHandler implements ICommandHandler")
	}

	// MultiBase should have: extends BaseClass, implements IFoo, implements ICommandHandler
	var multiDeps []parser.Dependency
	for _, dep := range deps {
		if dep.SourceSymbol == "MultiBase" {
			multiDeps = append(multiDeps, dep)
		}
	}
	if len(multiDeps) != 3 {
		t.Errorf("MultiBase should have 3 deps, got %d: %+v", len(multiDeps), multiDeps)
	}

	expectedDeps := map[string]string{
		"BaseClass":       "extends",
		"IFoo":            "implements",
		"ICommandHandler": "implements",
	}
	for _, dep := range multiDeps {
		if expected, ok := expectedDeps[dep.TargetName]; ok {
			if dep.Kind != expected {
				t.Errorf("MultiBase -> %s: got kind %q, want %q", dep.TargetName, dep.Kind, expected)
			}
		}
	}
}

func TestCSharpWalkerPropertyParentAssignment(t *testing.T) {
	source := `namespace MyApp
{
    public class User
    {
        public int Id { get; set; }
        public string Name { get; set; }
    }
}
`
	tree, src := parseCSharp(t, source)

	inputSymbols := []parser.Symbol{
		{Name: "User", Kind: "class", QualifiedName: "User", LineStart: 3, LineEnd: 7},
		{Name: "Id", Kind: "property", QualifiedName: "Id", LineStart: 5, LineEnd: 5},
		{Name: "Name", Kind: "property", QualifiedName: "Name", LineStart: 6, LineEnd: 6},
	}

	w := &CSharpWalker{}
	symbols, _ := w.Walk(tree, src, inputSymbols)

	for _, sym := range symbols {
		switch sym.Name {
		case "Id":
			if sym.ParentName != "User" {
				t.Errorf("Id parent: got %q, want %q", sym.ParentName, "User")
			}
			expected := "MyApp.User.Id"
			if sym.QualifiedName != expected {
				t.Errorf("Id qualified name: got %q, want %q", sym.QualifiedName, expected)
			}
		case "Name":
			if sym.ParentName != "User" {
				t.Errorf("Name parent: got %q, want %q", sym.ParentName, "User")
			}
		}
	}
}
