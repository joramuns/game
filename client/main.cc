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
            json j = json::parse(payload);
            std::cout << j << std::endl;
            for (const auto& item : j.items()) {
                std::cout << "Client ID: " << item.key() << ", Number: " << item.value()["number"] << std::endl;
            }
        } catch (const std::exception& e) {
            std::cout << "Error parsing message: " << e.what() << std::endl;
        }
    }

    client c;
    connection_hdl hdl;
    bool connected = false;
};

int main() {
    websocket_client ws_client;
    std::string uri = "ws://localhost:8080/ws";

    ws_client.connect(uri);

    while (true) {
        std::string input;
        std::cout << "Enter command (1: increment, 2: decrement, 0: exit): ";
        std::cin >> input;

        if (input == "1") {
            ws_client.send("increment");
        } else if (input == "2") {
            ws_client.send("decrement");
        } else if (input == "0") {
            ws_client.close();
            break;
        } else {
            std::cout << "Invalid command" << std::endl;
        }
    }

    return 0;
}

