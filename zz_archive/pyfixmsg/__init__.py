'''
pyfixmsg main init
'''
import sys
import itertools

import six

if six.PY2:
    STRSUM = lambda x: sum(bytearray(x))
else:
    STRSUM = sum


class RepeatingGroup(list):
    """ Implementation of repeating groups for pyfixmsg.FixMessage.
    The repeating group will look like {opening_tag:[FixMessage,FixMessage]} in the fix message
    a repeating group behaves like a list. You can add two repeating groups, or append a FixMessage to one.
    """

    def __init__(self, *args, **kwargs):
        """Maintains ``list``'s signature unchanged.

        Sets
        * self.number_tag (the tag that contains the number of elements in the group)
        * self.first_tag (the first repeated tag)
        * self.standard (reserved)
        """
        super(RepeatingGroup, self).__init__(*args, **kwargs)
        self.number_tag = None
        self.standard = True
        self.first_tag = None

    @property
    def entry_tag(self):
        """ returns the entry tag for the group and its value as a tuple"""
        return self.number_tag, len(self)

    @classmethod
    def create_repeating_group(cls, tag, standard=True, first_tag=None):
        """ creates a group with member. Can't use __init__ as it would mean overriding the list __init__ which sounds
        dangerous"""
        group = cls()
        group.number_tag = tag
        group.standard = standard
        group.first_tag = first_tag
        return group

    def __add__(self, other):
        """ addition of groups"""
        result = RepeatingGroupFactory(self.number_tag, self.standard,
                                       self.first_tag).get_r_group(*self)
        for group_member in other: # Changed 'group' to 'group_member' to avoid conflict if 'group' is passed to len_and_chsum
            result.append(group_member)
        return result

    def find_all(self, tag):
        """
        Generator.
        Find all instances of the tag in the message or inside repeating groups and returns the path to
        them one at a time.

        Example, navigate all paths for a given tag:
          >>> for path in msg.find_all(self, tag):
          ...   # path here is a list of ints or str keys
          ...   path_msg = msg
          ...   for key in path:
          ...     path_msg = path_msg[key]
          ...     # [...] do something at each level in the path
          ...   path_msg[tag] = # [...] do something with the last level of the path

        @return: a generator of paths where each path is a list of string or integer indices into the message
        @rtype: Generator of C{list} of C{int} or C{str}
        """
        for index, msg_item in enumerate(self): # Changed 'msg' to 'msg_item'
            if msg_item.anywhere(tag):
                for path in msg_item.find_all(tag):
                    result = [index]
                    result.extend(path)
                    yield result

    def all_tags(self):
        """
        Returns a list of all the tag keys in any member of this repeating group,
        The same tag will not appear twice in the generated sequence.
        The count tag for the repeating group is *not* included, it is considered as part of the
        parent message.
        Order is not guaranteed.
        @return: A list of tag keys (usually strings or ints)
        @rtype: C{list}
        """
        return list(set(tag for tag in itertools.chain(*(frag.all_tags() for frag in self))))

    def length(self): # This length is the FIX body length contribution, not list len()
        """
        Length of the body of the message in bytes
        """
        # Calls the global len_and_chsum for each member
        return sum(len_and_chsum(member, group=True)[0] for member in self)


class RepeatingGroupFactory(object):
    """ An easy way to create a repeating group for a given tag, without having to define all the tags yourself, takes
    the standard ones"""

    def __init__(self, tag, standard=True, first_tag=None):
        self.tag = tag
        self.standard = standard
        self.first_tag = first_tag

    def get_r_group(self, *fix_messages):
        """ factory method. I'm not familiar with the factory design pattern, it shows ;-)"""

        r_group = RepeatingGroup.create_repeating_group(self.tag, self.standard, self.first_tag)
        for fixmsg in fix_messages:
            r_group.append(fixmsg)
        return r_group

# --- NEW CHECKSUM AND LENGTH CALCULATION LOGIC ---
SEPARATOR = '\x01'
EQUALS_DELIMITER = '='

# Minimal header sort map (expand as needed, or ensure full import from .reference)
# Tags should be integers for keys here if tags in messages are primarily integers.
_MINIMAL_HEADER_SORT_MAP = {
    8: 1, 9: 2, 35: 3, 49: 4, 56: 5, 115: 5.1, 128: 5.2, # CompIDs
    50: 5.3, 142: 5.4, 57: 5.5, 143: 5.6, # OnBehalfOf/DeliverTo
    34: 6, 52: 7, 43: 8, 97: 9, 122: 10, # SeqNum, Time, PossDup etc.
    # Add other header tags with their desired sort priority
    # Standard header tags from pyfixmsg.reference (if available) would be better
    112: 11, # TestReqID
    108: 12, # HeartBtInt
    141: 13, # ResetSeqNumFlag
    # ... other common header tags
}

def _encode_value_for_checksum(val):
    """Encodes a field value to bytes for checksum calculation."""
    if isinstance(val, bytes):
        return val
    if isinstance(val, six.text_type):
        return val.encode('UTF-8')
    return str(val).encode('UTF-8')

def _get_ordered_items_for_checksum(items_dict, codec_spec_obj, type_spec_obj):
    """
    Orders items from items_dict based on FIX spec or fallback.
    Returns a list of (tag, value) tuples, ordered.
    """
    items_list = []
    # Ensure all keys in items_dict are consistently comparable (e.g. int or str)
    # For this function, we'll aim for integer tags where possible for sorting.
    processed_items = {}
    for k, v in items_dict.items():
        try:
            processed_items[int(str(k))] = v # Normalize to int key
        except ValueError:
            processed_items[k] = v # Keep original if not int-convertible (e.g. string for custom tags)
    
    items_list = list(processed_items.items())


    if type_spec_obj and hasattr(type_spec_obj, 'sorting_key') and type_spec_obj.sorting_key:
        spec_sorting_key_map = type_spec_obj.sorting_key
        def sort_key_func(item_tuple):
            tag = item_tuple[0]
            comparable_tag = int(tag) if not isinstance(tag, int) else tag
            return spec_sorting_key_map.get(comparable_tag, float('inf')) # Sort unknown tags to the end
        items_list.sort(key=sort_key_func)
    else:
        def fallback_sort_key_func(item_tuple):
            tag = item_tuple[0]
            numeric_tag = int(tag) if not isinstance(tag, int) else tag
            # Sort by header map, then by numeric tag value for non-header/unknown
            return (_MINIMAL_HEADER_SORT_MAP.get(numeric_tag, float('inf')), numeric_tag)
        items_list.sort(key=fallback_sort_key_func)
    return items_list

def _calculate_length_and_get_ordered_byte_parts(fragment_dict, codec_spec_obj, type_spec_obj):
    """
    Calculates body length for a FIX fragment (message body or group member)
    and returns its fields as a list of ordered byte strings ("tag=value<SOH>").
    """
    current_fragment_length = 0
    ordered_field_byte_strings = []

    ordered_items = _get_ordered_items_for_checksum(fragment_dict, codec_spec_obj, type_spec_obj)

    separator_bytes = SEPARATOR.encode('ascii')
    equals_bytes = EQUALS_DELIMITER.encode('ascii')

    for tag, value in ordered_items:
        encoded_tag = str(tag).encode('ascii') # Tags are numbers from sorting
        
        if isinstance(value, RepeatingGroup):
            num_in_group_tag_bytes = encoded_tag
            num_in_group_count_bytes = _encode_value_for_checksum(len(value))

            group_count_field_str = num_in_group_tag_bytes + equals_bytes + num_in_group_count_bytes + separator_bytes
            current_fragment_length += len(group_count_field_str)
            ordered_field_byte_strings.append(group_count_field_str)

            group_spec_for_members = None
            if type_spec_obj and hasattr(type_spec_obj, 'groups'):
                comparable_tag_for_group_lookup = int(tag) # tag is already int from _get_ordered_items
                group_spec_for_members = type_spec_obj.groups.get(comparable_tag_for_group_lookup)

            for member_fragment_dict in value:
                member_len, member_ordered_parts = \
                    _calculate_length_and_get_ordered_byte_parts(member_fragment_dict,
                                                                 codec_spec_obj,
                                                                 group_spec_for_members)
                current_fragment_length += member_len
                ordered_field_byte_strings.extend(member_ordered_parts)
        else:
            encoded_value = _encode_value_for_checksum(value)
            field_str = encoded_tag + equals_bytes + encoded_value + separator_bytes
            current_fragment_length += len(field_str)
            ordered_field_byte_strings.append(field_str)

    return current_fragment_length, ordered_field_byte_strings

def len_and_chsum(msg, group=False):
    """
    Calculate BodyLength and raw checksum value for a FIX message or fragment,
    respecting canonical FIX tag order for checksum calculation.
    - msg: The FixMessage or FixFragment (dict-like).
    - group: True if 'msg' is a member of a repeating group, False for top-level message.
    Returns: (body_length_int, raw_checksum_value_int)
    """
    separator_bytes = SEPARATOR.encode('ascii')
    equals_bytes = EQUALS_DELIMITER.encode('ascii')

    msg_codec = getattr(msg, 'codec', None)
    codec_spec_obj = getattr(msg_codec, 'spec', None) if msg_codec else None
    
    type_spec_obj_for_current_level = None
    if not group:
        if codec_spec_obj and hasattr(msg, 'get'):
            # Try to get MsgType, ensuring it's treated as a string for lookup in codec_spec_obj.msg_types
            msg_type_val_from_msg = msg.get(35) # Prefer int key first
            if msg_type_val_from_msg is None: msg_type_val_from_msg = msg.get('35')
            if msg_type_val_from_msg is None: msg_type_val_from_msg = msg.get(b'35')

            if msg_type_val_from_msg is not None and hasattr(codec_spec_obj, 'msg_types'):
                type_spec_obj_for_current_level = codec_spec_obj.msg_types.get(str(msg_type_val_from_msg))
    
    if group:
        fragment_len, fragment_ordered_parts = \
            _calculate_length_and_get_ordered_byte_parts(msg, codec_spec_obj, type_spec_obj_for_current_level) # type_spec_obj_for_current_level might be None
        
        raw_chsum_for_fragment = 0
        for part_bytes in fragment_ordered_parts:
            raw_chsum_for_fragment += STRSUM(part_bytes)
        return fragment_len, raw_chsum_for_fragment

    # --- Processing for a top-level message (group=False) ---
    tag_8_val = msg.get(8) # Prefer int key
    if tag_8_val is None: tag_8_val = msg.get('8')
    if tag_8_val is None: tag_8_val = msg.get(b'8')
        
    tag_8_byte_part = b''
    if tag_8_val is not None:
        tag_8_byte_part = b'8' + equals_bytes + _encode_value_for_checksum(tag_8_val) + separator_bytes
    else:
        pass # Invalid FIX message if Tag 8 is missing

    body_fields_dict = {}
    for tag_key, field_val in msg.items():
        tag_as_int = -1 
        if isinstance(tag_key, int): tag_as_int = tag_key
        else:
            try: tag_as_int = int(str(tag_key))
            except ValueError: pass

        if tag_as_int in (8, 9, 10): continue
        if isinstance(tag_key, (str, bytes)) and tag_key in ('8','9','10',b'8',b'9',b'10'): continue
        
        body_fields_dict[tag_key] = field_val

    calculated_body_length, ordered_body_byte_parts = \
        _calculate_length_and_get_ordered_byte_parts(body_fields_dict,
                                                     codec_spec_obj,
                                                     type_spec_obj_for_current_level)

    tag_9_byte_part = b'9' + equals_bytes + str(calculated_body_length).encode('ascii') + separator_bytes

    all_checksum_string_parts = []
    if tag_8_byte_part: 
        all_checksum_string_parts.append(tag_8_byte_part)
    all_checksum_string_parts.append(tag_9_byte_part)
    all_checksum_string_parts.extend(ordered_body_byte_parts)

    final_string_for_checksum = b''.join(all_checksum_string_parts)
    raw_checksum_value = STRSUM(final_string_for_checksum)

    # # ****** DEBUG PRINTS ******
    # # Get MsgType for logging, handling potential None
    # msg_type_for_log = "N/A"
    # msg_type_val_log = msg.get(35)
    # if msg_type_val_log is None: msg_type_val_log = msg.get('35')
    # if msg_type_val_log is None: msg_type_val_log = msg.get(b'35')
    # if msg_type_val_log is not None: msg_type_for_log = str(msg_type_val_log)

    # print(f"DEBUG pyfixmsg.len_and_chsum: ---- Initiator Calculation for MsgType {msg_type_for_log} ----")
    # # To avoid overly long prints for body_fields_dict if it contains complex objects:
    # # Convert values to simple string representations for logging if they are not basic types.
    # loggable_body_fields_dict = {}
    # for k, v in body_fields_dict.items():
    #     if isinstance(v, (str, int, float, bytes, bool)) or v is None:
    #         loggable_body_fields_dict[k] = v
    #     elif isinstance(v, RepeatingGroup):
    #         loggable_body_fields_dict[k] = f"<RepeatingGroup of {len(v)} items>"
    #     else:
    #         loggable_body_fields_dict[k] = f"<{type(v).__name__} object>"

    # print(f"DEBUG pyfixmsg.len_and_chsum: body_fields_dict passed to _calc: {loggable_body_fields_dict}")
    # print(f"DEBUG pyfixmsg.len_and_chsum: ordered_body_byte_parts (length {len(ordered_body_byte_parts)} items):")
    # for i, part in enumerate(ordered_body_byte_parts):
    #     # Represent part as string for logging, handling potential decoding errors
    #     try:
    #         part_repr = part.decode('utf-8', errors='replace')
    #     except AttributeError: # If part is not bytes
    #         part_repr = str(part)
    #     print(f"DEBUG pyfixmsg.len_and_chsum:   part[{i}] (len {len(part)}): {part_repr[:100]}{'...' if len(part_repr) > 100 else ''}")
    # print(f"DEBUG pyfixmsg.len_and_chsum: calculated_body_length: {calculated_body_length}")
    
    # # Represent final_string_for_checksum for logging
    # try:
    #     final_string_repr = final_string_for_checksum.decode('utf-8', errors='replace')
    # except AttributeError:
    #     final_string_repr = str(final_string_for_checksum)
    # print(f"DEBUG pyfixmsg.len_and_chsum: final_string_for_checksum (first 200 chars): {final_string_repr[:200]}{'...' if len(final_string_repr) > 200 else ''}")
    # print(f"DEBUG pyfixmsg.len_and_chsum: raw_checksum_value: {raw_checksum_value}")
    # print(f"DEBUG pyfixmsg.len_and_chsum: ---- End Initiator Calculation ----")
    # # ****** END DEBUG PRINTS ******

    return calculated_body_length, raw_checksum_value
# --- END OF NEW CHECKSUM AND LENGTH CALCULATION LOGIC ---
