-module(kafka_ffi).
-export([new_producer/1, produce/4, new_consumer/3, poll/1]).

new_producer(Brokers) ->
    % Check if brod is available, otherwise return dummy
    case code:which(brod) of
        non_existing -> {ok, dummy};
        _ ->
            BrokerList = parse_brokers(Brokers),
            case brod:start_client(BrokerList, realtime_gateway_client) of
                ok -> {ok, realtime_gateway_client};
                {error, {already_started, _}} -> {ok, realtime_gateway_client};
                {error, Reason} -> {error, {connection_error, Reason}}
            end
    end.

produce(Producer, Topic, Key, Value) ->
    case Producer of
        dummy -> {ok, nil};
        _ ->
            case brod:produce(Producer, Topic, Key, Key, Value) of
                {ok, _} -> {ok, nil};
                {error, Reason} -> {error, {publish_error, Reason}}
            end
    end.

new_consumer(Brokers, Topics, GroupId) ->
    case code:which(brod) of
        non_existing -> {ok, dummy};
        _ ->
            BrokerList = parse_brokers(Brokers),
            case brod:start_client(BrokerList, realtime_gateway_consumer_client) of
                ok -> ok;
                {error, {already_started, _}} -> ok;
                _ -> ok
            end,
            ConsumerConfig = [{begin_offset, earliest}],
            case brod:start_link_group_subscriber(
                realtime_gateway_consumer_client,
                GroupId,
                Topics,
                #{},
                ConsumerConfig,
                [],
                realtime_gateway_subscriber
            ) of
                {ok, Pid} -> {ok, Pid};
                {error, Reason} -> {error, {connection_error, Reason}}
            end
    end.

poll(Consumer) ->
    case Consumer of
        dummy -> [];
        _ -> []  % brod group subscriber uses callbacks, polling is async
    end.

parse_brokers(BrokersStr) ->
    lists:map(fun(Broker) ->
        [Host, PortStr] = string:tokens(Broker, ":"),
        {list_to_binary(Host), list_to_integer(PortStr)}
    end, string:tokens(BrokersStr, ",")).
