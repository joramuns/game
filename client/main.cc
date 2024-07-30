#include <iostream>
#include <nlohmann/json.hpp>
#include <string>
#include <thread>
#include <websocketpp/client.hpp>
#include <websocketpp/config/asio_no_tls_client.hpp>

typedef websocketpp::client<websocketpp::config::asio_client> client;

using json = nlohmann::json;
using websocketpp::connection_hdl;

class websocket_client {
 public:
  websocket_client() {
    c.init_asio();

    c.set_open_handler([this](connection_hdl hdl) {
      this->hdl = hdl;
      this->connected = true;
      std::cout << "Connected to server" << std::endl;
    });

    c.set_message_handler([this](connection_hdl, client::message_ptr msg) {
      this->on_message(msg->get_payload());
    });

    c.set_close_handler([this](connection_hdl) {
      this->connected = false;
      std::cout << "Disconnected from server" << std::endl;
    });
  }

  void connect(const std::string& uri) {
    websocketpp::lib::error_code ec;
    client::connection_ptr con = c.get_connection(uri, ec);
    if (ec) {
      std::cout << "Could not create connection because: " << ec.message()
                << std::endl;
      return;
    }

    c.connect(con);
    std::thread(&client::run, &c).detach();
  }

  void send(const std::string& message) {
    if (connected) {
      websocketpp::lib::error_code ec;
      c.send(hdl, message, websocketpp::frame::opcode::text, ec);
      if (ec) {
        std::cout << "Error sending message: " << ec.message() << std::endl;
      }
    } else {
      std::cout << "Not connected to server" << std::endl;
    }
  }

  void close() {
    if (connected) {
      websocketpp::lib::error_code ec;
      c.close(hdl, websocketpp::close::status::normal, "", ec);
      if (ec) {
        std::cout << "Error closing connection: " << ec.message() << std::endl;
      }
    }
  }

 private:
  void on_message(const std::string& payload) {
    try {
      json j = json::parse(payload);

      if (j.contains("client_id")) {
        int id = j["client_id"];
        client_id_ = std::to_string(id);
        std::cout << "Assigned Client ID: " << client_id_ << std::endl;
        return;
      }

      // Find client's own position
      if (j.contains(client_id_)) {
        client_x_ = j[client_id_]["x"];
        client_y_ = j[client_id_]["y"];
      }

      // Reset field to all zeros
      std::array<std::array<int, 7>, 7> field{};

      for (const auto& item : j.items()) {
        int x = item.value()["x"];
        int y = item.value()["y"];
        if (std::abs(client_x_ - x) <= 3 && std::abs(client_y_ - y) <= 3) {
          int display_x = 3 + (x - client_x_);
          int display_y = 3 + (y - client_y_);
          if (display_x >= 0 && display_x < 7 && display_y >= 0 &&
              display_y < 7) {
            field[display_y][display_x] = 1;
          }
        }
      }

      print_matrix(field);
    } catch (const std::exception& e) {
      std::cout << "Error parsing message: " << e.what() << std::endl;
    }
  }

  void print_matrix(const std::array<std::array<int, 7>, 7>& field) {
    for (const auto& row : field) {
      for (const auto& point : row) {
        std::cout << point;
      }
      std::cout << std::endl;
    }
  }

  client c;
  connection_hdl hdl;
  bool connected = false;
  std::string client_id_;
  int client_x_ = 0;
  int client_y_ = 0;
};

int main() {
  websocket_client ws_client;
  std::string uri = "ws://localhost:8080/ws";

  ws_client.connect(uri);

  while (true) {
    std::string input;
    std::cout << "Enter command (w: y+, s: y-, a: x-, d: x+, 0: exit): ";
    std::cin >> input;

    if (input == "d") {
      ws_client.send("x_plus");
    } else if (input == "a") {
      ws_client.send("x_minus");
    } else if (input == "w") {
      ws_client.send("y_minus");
    } else if (input == "s") {
      ws_client.send("y_plus");
    } else if (input == "0") {
      ws_client.close();
      break;
    } else {
      std::cout << "Invalid command" << std::endl;
    }
  }

  return 0;
}

