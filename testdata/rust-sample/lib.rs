use std::collections::HashMap;

pub trait Greeter {
    fn greet(&self) -> String;
}

pub struct User {
    pub name: String,
    pub age: u32,
}

impl User {
    pub fn new(name: String, age: u32) -> Self {
        User { name, age }
    }
}

impl Greeter for User {
    fn greet(&self) -> String {
        format!("Hello, I'm {}", self.name)
    }
}

pub fn create_user(name: &str, age: u32) -> User {
    User::new(name.to_string(), age)
}

pub enum UserRole {
    Admin,
    Editor,
    Viewer,
}
