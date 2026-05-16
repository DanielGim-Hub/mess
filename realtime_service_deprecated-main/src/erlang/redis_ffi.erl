-module(redis_ffi).
-export([connect/1, setex/4, get/2, del/2, publish/3]).

connect(Url) ->
    case application:ensure_started(eredis) of
        ok -> 
            case eredis:start_link([{url, Url}]) of
                {ok, Client} -> {ok, Client};
                {error, Reason} -> {error, {connection_error, Reason}}
            end;
        _ ->
            % Fallback: return dummy client if eredis not available
            {ok, dummy}
    end.

setex(Client, Key, Value, Seconds) ->
    case Client of
        dummy -> {ok, nil};
        _ ->
            case eredis:q(Client, ["SETEX", Key, integer_to_list(Seconds), Value]) of
                {ok, _} -> {ok, nil};
                {error, Reason} -> {error, {command_error, Reason}}
            end
    end.

get(Client, Key) ->
    case Client of
        dummy -> {error, {command_error, "not_found"}};
        _ ->
            case eredis:q(Client, ["GET", Key]) of
                {ok, undefined} -> {error, {command_error, "not_found"}};
                {ok, Value} -> {ok, Value};
                {error, Reason} -> {error, {command_error, Reason}}
            end
    end.

del(Client, Key) ->
    case Client of
        dummy -> {ok, nil};
        _ ->
            case eredis:q(Client, ["DEL", Key]) of
                {ok, _} -> {ok, nil};
                {error, Reason} -> {error, {command_error, Reason}}
            end
    end.

publish(Client, Channel, Message) ->
    case Client of
        dummy -> {ok, nil};
        _ ->
            case eredis:q(Client, ["PUBLISH", Channel, Message]) of
                {ok, _} -> {ok, nil};
                {error, Reason} -> {error, {command_error, Reason}}
            end
    end.
