#pragma once

#include <SDL2/SDL.h>

struct Player {
    int x, y;
    void draw(SDL_Renderer* renderer);
};

