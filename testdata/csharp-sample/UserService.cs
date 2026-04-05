using System;
using System.Collections.Generic;
using System.Linq;

namespace MyApp.Services
{
    public interface IUserService
    {
        User GetUser(int id);
        void AddUser(User user);
    }

    public class User
    {
        public int Id { get; set; }
        public string Name { get; set; }
        public string Email { get; set; }
    }

    public class UserService : IUserService
    {
        private readonly List<User> _users = new();

        public User GetUser(int id)
        {
            return _users.FirstOrDefault(u => u.Id == id);
        }

        public void AddUser(User user)
        {
            _users.Add(user);
        }

        public IEnumerable<User> FindByName(string name)
        {
            return _users.Where(u => u.Name == name);
        }
    }
}
