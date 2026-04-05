; C# tree-sitter queries for symbol and dependency extraction.

; --- Using directives (simple) ---
(using_directive
  (identifier) @definition.import.path)

; --- Using directives (qualified) ---
(using_directive
  (qualified_name) @definition.import.path)

; --- Class definitions ---
(class_declaration
  name: (identifier) @definition.class.name) @definition.class

; --- Interface definitions ---
(interface_declaration
  name: (identifier) @definition.interface.name) @definition.interface

; --- Method declarations (with block body) ---
(method_declaration
  name: (identifier) @definition.method.name
  parameters: (parameter_list) @definition.method.params) @definition.method

; --- Property declarations ---
(property_declaration
  name: (identifier) @definition.property.name) @definition.property

; --- Field declarations ---
(field_declaration
  (variable_declaration
    (variable_declarator
      (identifier) @definition.var.name))) @definition.var

; --- Qualified call expressions: obj.Method(args) ---
(invocation_expression
  function: (member_access_expression
    expression: (identifier) @reference.call.module
    name: (identifier) @reference.call.name)) @reference.call

; --- Simple call expressions: Method(args) ---
(invocation_expression
  function: (identifier) @reference.call.name) @reference.call.simple
