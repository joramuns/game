#include <iostream>
#include <string>
#include <thread>
#include <websocketpp/config/asio_no_tls_client.hpp>
#include <websocketpp/client.hpp>
#include <nlohmann/json.hpp>

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
            std::cout << "Could not create connection because: " << ec.message() << std::endl;
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
            std::array<std::array<int, 10>, 10> field{0};
            json j = json::parse(payload);
            std::cout << j << std::endl;
            for (const auto& item : j.items()) {
                int x = item.value()["x"];
                int y = item.value()["y"];
                std::cout << "Client ID: " << item.key() << ", X: " << x << ", Y: " << y << std::endl;
                field[y][x] = 1;
            }
            print_matrix(field);
        } catch (const std::exception& e) {
            std::cout << "Error parsing message: " << e.what() << std::endl;
        }
    }

    void print_matrix(const std::array<std::array<int, 10>, 10> &field) {
      for (const auto &row : field) {
        for (const auto &point : row) {
          std::cout << point; 
        }
        std::cout << std::endl;
      }
    }

    client c;
    connection_hdl hdl;
    bool connected = false;
    int client_id;
    int client_x;
    int client_y;
};

int main() {
    websocket_client ws_client;
    std::string uri = "ws://localhost:8080/ws";

    ws_client.connect(uri);

    while (true) {
        std::string input;
        std::cout << "Enter command (1: increment, 2: decrement, 0: exit): ";
        std::cin >> input;

        if (input == "d") {
            ws_client.send("x_plus");
        } else if (input == "a") {
            ws_client.send("x_minus");
        } else if (input == "w") {
            ws_client.send("y_plus");
        } else if (input == "s") {
            ws_client.send("y_minus");
        } else if (input == "0") {
            ws_client.close();
            break;
        } else {
            std::cout << "Invalid command" << std::endl;
        }
    }

    return 0;
}

