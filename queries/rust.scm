; Rust tree-sitter queries for symbol and dependency extraction.

; --- Use declarations (scoped path) ---
(use_declaration
  argument: (scoped_identifier) @definition.import.path)

; --- Use declarations (simple identifier) ---
(use_declaration
  argument: (identifier) @definition.import.path)

; --- Trait definitions ---
(trait_item
  name: (type_identifier) @definition.type.name) @definition.type.interface

; --- Struct definitions ---
(struct_item
  name: (type_identifier) @definition.type.name) @definition.type.struct

; --- Enum definitions ---
(enum_item
  name: (type_identifier) @definition.type.name) @definition.type.enum

; --- Function definitions (top-level and inside impl) ---
(function_item
  name: (identifier) @definition.function.name
  parameters: (parameters) @definition.function.params) @definition.function

; --- Trait method signatures (abstract methods inside traits) ---
(function_signature_item
  name: (identifier) @definition.method.name
  parameters: (parameters) @definition.method.params) @definition.method

; --- Qualified call expressions: Type::method(args) ---
(call_expression
  function: (scoped_identifier
    path: (identifier) @reference.call.module
    name: (identifier) @reference.call.name)) @reference.call

; --- Method call expressions: obj.method(args) ---
(call_expression
  function: (field_expression
    field: (field_identifier) @reference.call.name)) @reference.call.simple
