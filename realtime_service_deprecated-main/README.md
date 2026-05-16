# realtime gateway

realtime gateway это сервис доставки событий в реальном времени для чат-платформы.
принимает websocket-подключения клиентов, валидирует jwt при handshake через jwks и поддерживает одно активное соединение на пользователя.
сервис читает события из kafka, преобразует их в клиентские websocket-сообщения и отправляет их подключенным пользователям.
для ephemeral state он использует redis: presence, session routing и дедупликацию событий.
контракты websocket, redis, kafka и auth описаны в `docs/`.
