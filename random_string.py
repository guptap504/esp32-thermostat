from random import choice
from string import ascii_uppercase

print("".join(choice(ascii_uppercase) for _ in range(12)))
