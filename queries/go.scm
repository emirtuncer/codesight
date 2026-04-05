; Go tree-sitter queries for symbol and dependency extraction.
; Each pattern uses @capture names to identify what was matched.

; --- Functions ---
(function_declaration
  name: (identifier) @definition.function.name
  parameters: (parameter_list) @definition.function.params
  result: (_)? @definition.function.result
  body: (block) @definition.function.body) @definition.function

; --- Methods (with receiver) ---
(method_declaration
  receiver: (parameter_list) @definition.method.receiver
  name: (field_identifier) @definition.method.name
  parameters: (parameter_list) @definition.method.params
  body: (block) @definition.method.body) @definition.method

; --- Struct types ---
(type_declaration
  (type_spec
    name: (type_identifier) @definition.type.name
    type: (struct_type) @definition.type.body)) @definition.type.struct

; --- Interface types ---
(type_declaration
  (type_spec
    name: (type_identifier) @definition.type.name
    type: (interface_type) @definition.type.body)) @definition.type.interface

; --- Imports (grouped) ---
(import_declaration
  (import_spec_list
    (import_spec
      path: (interpreted_string_literal) @definition.import.path)))

; --- Imports (single) ---
(import_declaration
  (import_spec
    path: (interpreted_string_literal) @definition.import.path))

; --- Call expressions (qualified: pkg.Func) ---
(call_expression
  function: (selector_expression
    operand: (identifier) @reference.call.module
    field: (field_identifier) @reference.call.name)) @reference.call

; --- Call expressions (simple: Func) ---
(call_expression
  function: (identifier) @reference.call.name) @reference.call.simple

; --- Var declarations ---
(var_declaration
  (var_spec
    name: (identifier) @definition.var.name)) @definition.var

; --- Const declarations ---
(const_declaration
  (const_spec
    name: (identifier) @definition.const.name)) @definition.const
