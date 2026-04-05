package com.example.service;

import java.util.ArrayList;
import java.util.List;
import java.util.Optional;

public interface UserRepository {
    User findById(int id);
    void save(User user);
}

public class User {
    private String name;
    private String email;

    public User(String name, String email) {
        this.name = name;
        this.email = email;
    }

    public String getName() { return name; }
    public String getEmail() { return email; }
}

public class UserService implements UserRepository {
    private final List<User> users = new ArrayList<>();

    @Override
    public User findById(int id) {
        return users.get(id);
    }

    @Override
    public void save(User user) {
        users.add(user);
    }

    public Optional<User> findByName(String name) {
        return users.stream()
            .filter(u -> u.getName().equals(name))
            .findFirst();
    }
}
