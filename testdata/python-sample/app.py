import os
from typing import List, Optional
from dataclasses import dataclass

@dataclass
class User:
    name: str
    email: str
    age: int = 0

class UserService:
    def __init__(self):
        self._users: List[User] = []

    def add_user(self, user: User) -> None:
        self._users.append(user)

    def find_by_name(self, name: str) -> Optional[User]:
        for user in self._users:
            if user.name == name:
                return user
        return None

def create_user(name: str, email: str) -> User:
    return User(name=name, email=email)

API_VERSION = "1.0"
