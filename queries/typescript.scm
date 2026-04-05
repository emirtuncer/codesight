; TypeScript tree-sitter queries for symbol and dependency extraction.
; Capture names are chosen to work with the existing QueryRunner.processMatch logic,
; with new TS-specific types where needed.

; --- Function declarations ---
(function_declaration
  name: (identifier) @definition.function.name
  parameters: (formal_parameters) @definition.function.params) @definition.function

; --- Class declarations ---
(class_declaration
  name: (type_identifier) @definition.class.name) @definition.class

; --- Method definitions (inside class bodies) ---
(method_definition
  name: (property_identifier) @definition.method.name
  parameters: (formal_parameters) @definition.method.params) @definition.method

; --- Interface declarations ---
(interface_declaration
  name: (type_identifier) @definition.interface.name) @definition.interface

; --- Type alias declarations ---
(type_alias_declaration
  name: (type_identifier) @definition.type_alias.name) @definition.type_alias

; --- Import statements ---
(import_statement
  source: (string) @definition.import.path)

; --- Const/let/var declarations (lexical_declaration) ---
(lexical_declaration
  (variable_declarator
    name: (identifier) @definition.var.name)) @definition.var

; --- Call expressions (method calls: obj.method()) ---
(call_expression
  function: (member_expression
    object: (identifier) @reference.call.module
    property: (property_identifier) @reference.call.name)) @reference.call

; --- Call expressions (simple: func()) ---
(call_expression
  function: (identifier) @reference.call.name) @reference.call.simple
