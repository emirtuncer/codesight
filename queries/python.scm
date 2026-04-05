; Python tree-sitter queries for symbol and dependency extraction.

; --- Function definitions (top-level and nested) ---
(function_definition
  name: (identifier) @definition.function.name
  parameters: (parameters) @definition.function.params
  return_type: (type)? @definition.function.result) @definition.function

; --- Class definitions ---
(class_definition
  name: (identifier) @definition.class.name) @definition.class

; --- Decorated definitions (class or function with decorator) ---
(decorated_definition
  (decorator) @definition.decorator
  (class_definition
    name: (identifier) @definition.decorated_class.name)) @definition.decorated_class

(decorated_definition
  (decorator) @definition.decorator.func
  (function_definition
    name: (identifier) @definition.decorated_function.name
    parameters: (parameters) @definition.decorated_function.params
    return_type: (type)? @definition.decorated_function.result)) @definition.decorated_function

; --- Import statements: import X ---
(import_statement
  name: (dotted_name) @definition.import.path)

; --- From-import statements: from X import Y ---
(import_from_statement
  module_name: (dotted_name) @definition.import.path)

; --- Top-level variable assignments ---
(expression_statement
  (assignment
    left: (identifier) @definition.var.name)) @definition.var

; --- Call expressions (simple: func()) ---
(call
  function: (identifier) @reference.call.name) @reference.call.simple

; --- Call expressions (attribute: obj.method()) ---
(call
  function: (attribute
    object: (identifier) @reference.call.module
    attribute: (identifier) @reference.call.name)) @reference.call
