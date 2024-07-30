CXX = g++
CXXFLAGS = -std=c++11 -lboost_system -I include/
LDFLAGS = -lboost_system 

all: game-client

game-client: main.o
	$(CXX) -o game-client main.o $(LDFLAGS)

main.o: main.cc player.h
	$(CXX) $(CXXFLAGS) -c main.cc

clean:
	rm -f *.o game-client

