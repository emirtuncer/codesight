; Java tree-sitter queries for symbol and dependency extraction.

; --- Package declaration ---
(package_declaration
  (scoped_identifier) @definition.import.path)

; --- Import declarations ---
(import_declaration
  (scoped_identifier) @definition.import.path)

; --- Interface definitions ---
(interface_declaration
  name: (identifier) @definition.interface.name) @definition.interface

; --- Class definitions ---
(class_declaration
  name: (identifier) @definition.class.name) @definition.class

; --- Method declarations ---
(method_declaration
  name: (identifier) @definition.method.name
  parameters: (formal_parameters) @definition.method.params) @definition.method

; --- Constructor declarations ---
(constructor_declaration
  name: (identifier) @definition.function.name
  parameters: (formal_parameters) @definition.function.params) @definition.function

; --- Field declarations ---
(field_declaration
  (variable_declarator
    name: (identifier) @definition.var.name)) @definition.var

; --- Qualified method invocations: obj.method(args) ---
(method_invocation
  object: (identifier) @reference.call.module
  name: (identifier) @reference.call.name) @reference.call

; --- Simple method invocations: method(args) ---
(method_invocation
  name: (identifier) @reference.call.name
  arguments: (argument_list)) @reference.call.simple
